package site

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// Additional helper methods for site operations

// EnableSite enables a site by creating a symlink
func (sm *CaddySiteManager) EnableSite(domain string) error {
	if sm.Config.Verbose {
		fmt.Printf("Enabling site: %s\n", domain)
	}

	configFile := filepath.Join(sm.Config.AvailableSites, domain)
	symlinkPath := filepath.Join(sm.Config.EnabledSites, domain)

	// In dry-run mode, skip file existence check
	if !sm.Config.DryRun {
		// Check if config file exists
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			return fmt.Errorf("site configuration not found: %s", domain)
		}

		// Check if already enabled
		if _, err := os.Lstat(symlinkPath); err == nil {
			if sm.Config.Verbose {
				fmt.Printf("Site %s is already enabled\n", domain)
			}
			return nil
		}
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would create symlink: %s -> %s\n", symlinkPath, configFile)
		}
		return nil
	}

	// Create symlink
	if err := os.Symlink(configFile, symlinkPath); err != nil {
		return fmt.Errorf("failed to create symlink: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Printf("Site %s enabled successfully\n", domain)
	}

	return nil
}

// DisableSite disables a site by removing the symlink
func (sm *CaddySiteManager) DisableSite(domain string) error {
	if sm.Config.Verbose {
		fmt.Printf("Disabling site: %s\n", domain)
	}

	symlinkPath := filepath.Join(sm.Config.EnabledSites, domain)

	// Check if symlink exists
	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		return fmt.Errorf("site %s is not enabled", domain)
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would remove symlink: %s\n", symlinkPath)
		}
		return nil
	}

	// Remove symlink
	if err := os.Remove(symlinkPath); err != nil {
		return fmt.Errorf("failed to remove symlink: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Printf("Site %s disabled successfully\n", domain)
	}

	return nil
}

// ListSites lists all available and enabled sites
func (sm *CaddySiteManager) ListSites() error {
	// List available sites
	fmt.Println("Available sites:")
	availableFiles, err := filepath.Glob(filepath.Join(sm.Config.AvailableSites, "*"))
	if err != nil {
		return fmt.Errorf("failed to list available sites: %v", err)
	}

	for _, file := range availableFiles {
		if !strings.HasSuffix(file, ".conf") { // Skip .conf extension
			domain := filepath.Base(file)
			fmt.Printf("  %s\n", domain)
		}
	}

	// List enabled sites
	fmt.Println("\nEnabled sites:")
	enabledFiles, err := filepath.Glob(filepath.Join(sm.Config.EnabledSites, "*"))
	if err != nil {
		return fmt.Errorf("failed to list enabled sites: %v", err)
	}

	for _, file := range enabledFiles {
		if !strings.HasSuffix(file, ".conf") { // Skip .conf extension
			domain := filepath.Base(file)
			fmt.Printf("  %s\n", domain)
		}
	}

	return nil
}

// validateAndReloadCaddy validates and reloads the Caddy configuration
func (sm *CaddySiteManager) validateAndReloadCaddy() error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Println("Would validate and reload Caddy configuration")
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Println("Testing Caddy configuration...")
	}

	// Validate Caddy configuration
	cmd := exec.Command("caddy", "validate", "--config", "/etc/caddy/Caddyfile")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("caddy configuration validation failed: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Println("Caddy configuration is valid.")
	}

	// Reload Caddy
	return sm.reloadCaddy()
}

// reloadCaddy reloads the Caddy service
func (sm *CaddySiteManager) reloadCaddy() error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Println("Would reload Caddy")
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Println("Reloading Caddy...")
	}

	cmd := exec.Command("systemctl", "reload", "caddy")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Println("Caddy reloaded successfully.")
	}

	return nil
}

// getSiteInfo extracts site information from config file
func (sm *CaddySiteManager) getSiteInfo(domain string) (*CaddySite, error) {
	configFile := filepath.Join(sm.Config.AvailableSites, domain)
	
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("site config not found: %s", domain)
	}

	// Try to extract document root from config file
	documentRoot, err := sm.extractDocumentRoot(configFile, domain)
	if err != nil {
		if sm.Config.Verbose {
			fmt.Printf("Failed to extract document root from config: %v\n", err)
			fmt.Printf("Using fallback method...\n")
		}
		
		// Fallback: use standard directory structure
		documentRoot = filepath.Join("/var/www/sites", domain)
		
		// Verify the directory exists
		if _, err := os.Stat(documentRoot); os.IsNotExist(err) {
			// Try alternative web root from config
			if sm.Config.WebRoot != "" {
				documentRoot = filepath.Join(sm.Config.WebRoot, "sites", domain)
				if _, err := os.Stat(documentRoot); os.IsNotExist(err) {
					return nil, fmt.Errorf("could not find document root for domain %s. Tried: /var/www/sites/%s and %s/sites/%s", 
						domain, domain, sm.Config.WebRoot, domain)
				}
			} else {
				return nil, fmt.Errorf("could not find document root for domain %s. Directory /var/www/sites/%s does not exist", domain, domain)
			}
		}
		
		if sm.Config.Verbose {
			fmt.Printf("Using fallback document root: %s\n", documentRoot)
		}
	}

	// Check if it's a WordPress site
	isWordPress := false
	wpConfigPath := filepath.Join(documentRoot, "wp-config.php")
	if _, err := os.Stat(wpConfigPath); err == nil {
		isWordPress = true
	}

	poolName := generatePoolName(domain)

	return &CaddySite{
		Domain:       domain,
		DocumentRoot: documentRoot,
		IsWordPress:  isWordPress,
		PoolName:     poolName,
		ConfigFile:   configFile,
	}, nil
}

// extractDocumentRoot extracts the document root from a Caddy config file
func (sm *CaddySiteManager) extractDocumentRoot(configFile, domain string) (string, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if sm.Config.Verbose {
		fmt.Printf("Parsing config file: %s for domain: %s\n", configFile, domain)
	}

	scanner := bufio.NewScanner(file)
	inDomainBlock := false
	braceCount := 0
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		if sm.Config.Verbose && sm.Config.DryRun {
			fmt.Printf("Line %d: %s\n", lineNum, line)
		}
		
		// Check if we're entering the domain block (only if not already in one)
		if !inDomainBlock && strings.HasPrefix(line, domain) && (strings.Contains(line, "{") || strings.HasSuffix(line, domain)) {
			inDomainBlock = true
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")
			if sm.Config.Verbose {
				fmt.Printf("Found domain block for %s at line %d (braces: %d)\n", domain, lineNum, braceCount)
			}
			continue
		}

		if inDomainBlock {
			// Count braces
			openBraces := strings.Count(line, "{")
			closeBraces := strings.Count(line, "}")
			braceCount += openBraces - closeBraces
			
			if sm.Config.Verbose && sm.Config.DryRun {
				fmt.Printf("  In domain block, braces: %d\n", braceCount)
			}
			
			// Look for root directive
			if strings.HasPrefix(line, "root ") || strings.Contains(line, "root ") {
				parts := strings.Fields(line)
				if sm.Config.Verbose {
					fmt.Printf("Found root directive at line %d: %v\n", lineNum, parts)
				}
				if len(parts) >= 3 && parts[1] == "*" {
					if sm.Config.Verbose {
						fmt.Printf("Extracted document root: %s\n", parts[2])
					}
					return parts[2], nil
				} else if len(parts) >= 2 {
					if sm.Config.Verbose {
						fmt.Printf("Extracted document root: %s\n", parts[1])
					}
					return parts[1], nil
				}
			}
			
			// Exit domain block when braces are balanced
			if braceCount <= 0 {
				inDomainBlock = false
				if sm.Config.Verbose {
					fmt.Printf("Exiting domain block at line %d\n", lineNum)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading config file: %v", err)
	}

	return "", fmt.Errorf("could not find root directive for domain %s", domain)
}

// removePHPFPMPool removes a PHP-FPM pool
func (sm *CaddySiteManager) removePHPFPMPool(site *CaddySite) error {
	poolConfigFile := fmt.Sprintf("/etc/php/%s/fpm/pool.d/%s.conf", site.PHPVersion, site.PoolName)
	poolLogFile := fmt.Sprintf("/var/log/php/%s-error.log", site.PoolName)
	
	if sm.Config.Verbose {
		fmt.Printf("Checking for custom PHP-FPM pool: %s\n", site.PoolName)
	}

	// Remove pool config if it exists
	if _, err := os.Stat(poolConfigFile); err == nil {
		if sm.Config.DryRun {
			if sm.Config.Verbose {
				fmt.Printf("Would remove PHP-FPM pool: %s\n", poolConfigFile)
			}
		} else {
			if sm.Config.Verbose {
				fmt.Printf("Removing PHP-FPM pool configuration: %s\n", poolConfigFile)
			}
			if err := os.Remove(poolConfigFile); err != nil {
				return fmt.Errorf("failed to remove pool config: %v", err)
			}

			// Remove log file if it exists
			if _, err := os.Stat(poolLogFile); err == nil {
				if sm.Config.Verbose {
					fmt.Printf("Removing PHP-FPM pool log file: %s\n", poolLogFile)
				}
				os.Remove(poolLogFile) // Don't fail if log removal fails
			}

			// Restart PHP-FPM
			if err := sm.restartPHPFPM(site.PHPVersion); err != nil {
				return fmt.Errorf("failed to restart PHP-FPM: %v", err)
			}
		}
	} else {
		if sm.Config.Verbose {
			fmt.Printf("No custom PHP-FPM pool found for domain '%s'\n", site.Domain)
		}
	}

	return nil
}

// removeSymlink removes a symlink
func (sm *CaddySiteManager) removeSymlink(symlinkPath string) error {
	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		if sm.Config.Verbose {
			fmt.Printf("Symlink not found: %s\n", symlinkPath)
		}
		return nil
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would remove symlink: %s\n", symlinkPath)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Printf("Removing symlink: %s\n", symlinkPath)
	}

	if err := os.Remove(symlinkPath); err != nil {
		return fmt.Errorf("failed to remove symlink: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Println("Symlink removed successfully")
	}

	return nil
}

// removeFile removes a file
func (sm *CaddySiteManager) removeFile(filePath, description string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if sm.Config.Verbose {
			fmt.Printf("%s not found: %s\n", description, filePath)
		}
		return nil
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would remove %s: %s\n", description, filePath)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Printf("Deleting %s: %s\n", description, filePath)
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to remove %s: %v", description, err)
	}

	if sm.Config.Verbose {
		fmt.Printf("%s deleted successfully\n", description)
	}

	return nil
}

// removeDirectory removes a directory recursively
func (sm *CaddySiteManager) removeDirectory(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if sm.Config.Verbose {
			fmt.Printf("Directory not found: %s\n", dirPath)
		}
		return nil
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would remove directory: %s\n", dirPath)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Printf("Deleting web directory '%s'...\n", dirPath)
	}

	if err := os.RemoveAll(dirPath); err != nil {
		return fmt.Errorf("failed to delete directory '%s': %v", dirPath, err)
	}

	if sm.Config.Verbose {
		fmt.Println("Directory deleted successfully")
	}

	return nil
}

// printSuccessMessage prints the success message after site creation
func (sm *CaddySiteManager) printSuccessMessage(site *CaddySite) {
	siteType := "PHP"
	if site.IsWordPress {
		siteType = "WordPress"
	}

	fmt.Println("")
	fmt.Println("============================================")
	fmt.Printf("%s site setup complete!\n", siteType)
	fmt.Println("============================================")
	fmt.Printf("Domain: %s\n", site.Domain)
	fmt.Printf("Site directory: %s\n", site.DocumentRoot)
	fmt.Printf("PHP-FPM Pool: %s\n", site.PoolName)
	fmt.Printf("PHP-FPM Socket: /run/php/php%s-fpm-%s.sock\n", site.PHPVersion, site.PoolName)
	fmt.Printf("Configuration: %s\n", site.ConfigFile)
	fmt.Printf("Enabled via: %s\n", filepath.Join(sm.Config.EnabledSites, site.Domain))

	if site.IsWordPress {
		fmt.Printf("Database: %s\n", site.DBName)
		fmt.Printf("Database user: %s\n", site.DBUser)
		fmt.Printf("Database password: %s\n", site.DBPassword)
	}

	fmt.Println("")
	fmt.Println("PHP settings:")
	fmt.Printf("  upload_max_filesize: %s\n", site.MaxUpload)
	fmt.Printf("  post_max_size: %s\n", site.MaxUpload)
	fmt.Println("  memory_limit: 512M")
	fmt.Println("  max_execution_time: 300s")
	fmt.Println("  max_input_vars: 5000")
	fmt.Println("")
	fmt.Println("Caddy has been configured and reloaded.")

	if site.IsWordPress {
		fmt.Printf("Visit https://%s to complete WordPress installation\n", site.Domain)
		fmt.Println("")
		fmt.Println("Database credentials for WordPress installation:")
		fmt.Printf("  Database Name: %s\n", site.DBName)
		fmt.Printf("  Username: %s\n", site.DBUser)
		fmt.Printf("  Password: %s\n", site.DBPassword)
		fmt.Println("  Database Host: localhost")
	} else {
		fmt.Printf("Visit https://%s to view your PHP site\n", site.Domain)
	}
}

// Missing methods implementations

// checkConflicts checks for existing site conflicts
func (sm *CaddySiteManager) checkConflicts(site *CaddySite) error {
	// Check if site directory already exists
	if _, err := os.Stat(site.DocumentRoot); err == nil {
		if !sm.Config.DryRun {
			if !sm.confirmOverwrite(fmt.Sprintf("Site directory '%s' already exists", site.DocumentRoot)) {
				return fmt.Errorf("aborting site setup")
			}
			if sm.Config.Verbose {
				fmt.Println("Removing existing site directory...")
			}
			if err := os.RemoveAll(site.DocumentRoot); err != nil {
				return fmt.Errorf("failed to remove existing directory: %v", err)
			}
		}
	}

	// Check if config file already exists
	if _, err := os.Stat(site.ConfigFile); err == nil {
		if !sm.Config.DryRun {
			if !sm.confirmOverwrite(fmt.Sprintf("Domain configuration '%s' already exists", site.Domain)) {
				return fmt.Errorf("aborting site setup")
			}
			if sm.Config.Verbose {
				fmt.Println("Removing existing configuration...")
			}
			// Remove both config and symlink
			os.Remove(site.ConfigFile)
			os.Remove(filepath.Join(sm.Config.EnabledSites, site.Domain))
		}
	}

	// For WordPress sites, check database conflicts
	if site.IsWordPress {
		if err := sm.checkDatabaseConflicts(site); err != nil {
			return err
		}
	}

	return nil
}

// checkDatabaseConflicts checks for existing database conflicts
func (sm *CaddySiteManager) checkDatabaseConflicts(site *CaddySite) error {
	if sm.Config.DryRun {
		return nil
	}

	// Check if database exists
	dbExists, err := sm.databaseExists(site.DBName)
	if err != nil {
		return fmt.Errorf("failed to check database existence: %v", err)
	}

	if dbExists {
		if !sm.confirmOverwrite(fmt.Sprintf("Database '%s' already exists", site.DBName)) {
			return fmt.Errorf("aborting site setup")
		}
		if sm.Config.Verbose {
			fmt.Println("Dropping existing database...")
		}
		if err := sm.dropDatabase(site.DBName); err != nil {
			return fmt.Errorf("failed to drop existing database: %v", err)
		}
	}

	// Check if database user exists
	userExists, err := sm.databaseUserExists(site.DBUser)
	if err != nil {
		return fmt.Errorf("failed to check database user existence: %v", err)
	}

	if userExists {
		if !sm.confirmOverwrite(fmt.Sprintf("Database user '%s' already exists", site.DBUser)) {
			fmt.Println("Note: Continuing with existing user. Make sure the password is correct.")
		} else {
			if sm.Config.Verbose {
				fmt.Println("Dropping existing database user...")
			}
			if err := sm.dropDatabaseUser(site.DBUser); err != nil {
				return fmt.Errorf("failed to drop existing database user: %v", err)
			}
		}
	}

	return nil
}

// createPHPFPMPool creates a custom PHP-FPM pool for the site
func (sm *CaddySiteManager) createPHPFPMPool(site *CaddySite) error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would create PHP-FPM pool: %s\n", site.PoolName)
		}
		return nil
	}

	poolConfigFile := fmt.Sprintf("/etc/php/%s/fpm/pool.d/%s.conf", site.PHPVersion, site.PoolName)
	
	if sm.Config.Verbose {
		fmt.Printf("Creating PHP-FPM pool configuration for %s...\n", site.Domain)
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll("/var/log/php", 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Execute chown command to set ownership
	if err := exec.Command("chown", "www-data:www-data", "/var/log/php").Run(); err != nil {
		return fmt.Errorf("failed to set log directory ownership: %v", err)
	}

	// Generate PHP-FPM pool configuration
	file, err := os.Create(poolConfigFile)
	if err != nil {
		return fmt.Errorf("failed to create pool config file: %v", err)
	}
	defer file.Close()

	return sm.phpPoolTmpl.Execute(file, site)
}

// restartPHPFPM restarts PHP-FPM to load the new pool
func (sm *CaddySiteManager) restartPHPFPM(phpVersion string) error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would restart PHP-FPM %s\n", phpVersion)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Printf("Restarting PHP-FPM to load the new pool...\n")
	}

	serviceName := fmt.Sprintf("php%s-fpm", phpVersion)
	cmd := exec.Command("systemctl", "restart", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart PHP-FPM: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Println("PHP-FPM restarted successfully.")
	}

	return nil
}

// createSiteDirectory creates the site directory structure
func (sm *CaddySiteManager) createSiteDirectory(site *CaddySite) error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would create site directory: %s\n", site.DocumentRoot)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Println("Creating site directory...")
	}

	if err := os.MkdirAll(site.DocumentRoot, 0755); err != nil {
		return fmt.Errorf("failed to create site directory: %v", err)
	}

	return nil
}

// createBasicPHPSite creates a basic PHP site structure
func (sm *CaddySiteManager) createBasicPHPSite(site *CaddySite) error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would create basic PHP site files in: %s\n", site.DocumentRoot)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Println("Creating basic PHP site structure...")
	}

	indexContent := fmt.Sprintf(`<?php
echo "<h1>Welcome to %s</h1>";
echo "<p>This is a basic PHP site.</p>";
echo "<p>PHP Version: " . phpversion() . "</p>";
echo "<p>Server Time: " . date('Y-m-d H:i:s') . "</p>";
?>`, site.Domain)

	indexFile := filepath.Join(site.DocumentRoot, "index.php")
	if err := os.WriteFile(indexFile, []byte(indexContent), 0644); err != nil {
		return fmt.Errorf("failed to create index.php: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Println("Basic PHP site files created")
	}

	return nil
}

// createWordPressSite creates a WordPress site
func (sm *CaddySiteManager) createWordPressSite(site *CaddySite) error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would create WordPress site in: %s\n", site.DocumentRoot)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Println("Creating WordPress site...")
	}

	// Copy WordPress template
	templateDir := "/var/www/sites/wordpress-template"
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		return fmt.Errorf("WordPress template not found at %s. Please ensure the template directory exists with a WordPress installation", templateDir)
	}

	if sm.Config.Verbose {
		fmt.Println("Copying WordPress template...")
	}

	// Copy template files
	if err := sm.copyDir(templateDir, site.DocumentRoot); err != nil {
		return fmt.Errorf("failed to copy WordPress template: %v", err)
	}

	// Create database and user
	if err := sm.setupWordPressDatabase(site); err != nil {
		return err
	}

	// Generate wp-config.php
	if err := sm.generateWordPressConfig(site); err != nil {
		return err
	}

	if sm.Config.Verbose {
		fmt.Println("WordPress configuration created")
	}

	return nil
}

// setupWordPressDatabase creates the database and user for WordPress
func (sm *CaddySiteManager) setupWordPressDatabase(site *CaddySite) error {
	if sm.Config.Verbose {
		fmt.Println("Setting up database and user...")
	}

	queries := []string{
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", site.DBName),
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';", site.DBUser, site.DBPassword),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';", site.DBName, site.DBUser),
		"FLUSH PRIVILEGES;",
	}

	for _, query := range queries {
		cmd := exec.Command("mysql", "-u", "root", "-e", query)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to execute database query: %v", err)
		}
	}

	return nil
}

// generateWordPressConfig generates wp-config.php for WordPress
func (sm *CaddySiteManager) generateWordPressConfig(site *CaddySite) error {
	// Get WordPress salts
	saltKeys, err := sm.getWordPressSalts()
	if err != nil {
		return fmt.Errorf("failed to get WordPress salts: %v", err)
	}

	wpConfigContent := fmt.Sprintf(`<?php
define( 'DB_NAME', '%s' );
define( 'DB_USER', '%s' );
define( 'DB_PASSWORD', '%s' );
define( 'DB_HOST', 'localhost' );
define( 'DB_CHARSET', 'utf8mb4' );
define( 'DB_COLLATE', '' );

%s

$table_prefix = 'wp_';

define( 'WP_DEBUG', false );

if ( ! defined( 'ABSPATH' ) ) {
    define( 'ABSPATH', __DIR__ . '/' );
}

require_once ABSPATH . 'wp-settings.php';
`, site.DBName, site.DBUser, site.DBPassword, saltKeys)

	wpConfigFile := filepath.Join(site.DocumentRoot, "wp-config.php")
	if err := os.WriteFile(wpConfigFile, []byte(wpConfigContent), 0600); err != nil {
		return fmt.Errorf("failed to create wp-config.php: %v", err)
	}

	return nil
}

// setPermissions sets proper file permissions for the site
func (sm *CaddySiteManager) setPermissions(site *CaddySite) error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would set permissions for: %s\n", site.DocumentRoot)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Println("Setting file permissions...")
	}

	// Set ownership
	if err := exec.Command("chown", "-R", "www-data:www-data", site.DocumentRoot).Run(); err != nil {
		return fmt.Errorf("failed to set ownership: %v", err)
	}

	// Set directory permissions
	if err := exec.Command("find", site.DocumentRoot, "-type", "d", "-exec", "chmod", "755", "{}", "+").Run(); err != nil {
		return fmt.Errorf("failed to set directory permissions: %v", err)
	}

	// Set file permissions
	if err := exec.Command("find", site.DocumentRoot, "-type", "f", "-exec", "chmod", "644", "{}", "+").Run(); err != nil {
		return fmt.Errorf("failed to set file permissions: %v", err)
	}

	// Set special permissions for wp-config.php if it exists
	wpConfigFile := filepath.Join(site.DocumentRoot, "wp-config.php")
	if _, err := os.Stat(wpConfigFile); err == nil {
		if err := os.Chmod(wpConfigFile, 0600); err != nil {
			return fmt.Errorf("failed to set wp-config.php permissions: %v", err)
		}
	}

	return nil
}

// generateCaddyConfig generates the Caddy configuration for the site
func (sm *CaddySiteManager) generateCaddyConfig(site *CaddySite) error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would create Caddy config: %s\n", site.ConfigFile)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Printf("Creating Caddy configuration for %s...\n", site.Domain)
	}

	file, err := os.Create(site.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer file.Close()

	var tmpl *template.Template
	if site.IsWordPress {
		tmpl = sm.wpTmpl
	} else {
		tmpl = sm.caddyTmpl
	}

	return tmpl.Execute(file, site)
}

// Helper methods for SiteManager

// confirmOverwrite prompts the user for confirmation
func (sm *CaddySiteManager) confirmOverwrite(message string) bool {
	fmt.Printf("Warning: %s.\n", message)
	fmt.Print("Do you want to overwrite? (y/n): ")
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

// getWordPressSalts retrieves WordPress security salts from the API
func (sm *CaddySiteManager) getWordPressSalts() (string, error) {
	resp, err := http.Get("https://api.wordpress.org/secret-key/1.1/salt/")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	salts, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(salts), nil
}

// copyDir recursively copies a directory
func (sm *CaddySiteManager) copyDir(src, dst string) error {
	return exec.Command("cp", "-R", src+"/.", dst+"/").Run()
}

// AddBasicAuth adds basic authentication to a specific path in a site
func (sm *CaddySiteManager) AddBasicAuth(domain, path, username, password string) error {
	if sm.Config.Verbose {
		fmt.Printf("Adding basic auth for %s to path %s\n", domain, path)
	}

	// Validate inputs
	if domain == "" || path == "" || username == "" || password == "" {
		return fmt.Errorf("domain, path, username, and password are all required")
	}

	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	configFile := filepath.Join(sm.Config.AvailableSites, domain)
	
	// Check if site exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("site '%s' not found", domain)
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would add basic auth:\n")
			fmt.Printf("  Domain: %s\n", domain)
			fmt.Printf("  Path: %s\n", path)
			fmt.Printf("  Username: %s\n", username)
			fmt.Printf("  Password: %s\n", password)
		}
		return nil
	}

	// Read current config
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// Generate password hash using Caddy's bcrypt
	hashedPassword, err := sm.generatePasswordHash(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Add basic auth block
	authBlock := fmt.Sprintf(`
	# Basic auth for %s
	@auth_%s {
		path %s*
	}
	basic_auth @auth_%s {
		%s %s
	}`, path, sm.sanitizeName(path), path, sm.sanitizeName(path), username, hashedPassword)

	// Insert auth block before the PHP processing
	configStr := string(content)
	phpIndex := strings.Index(configStr, "php_fastcgi")
	if phpIndex == -1 {
		return fmt.Errorf("could not find PHP configuration in site config")
	}

	// Insert auth block before PHP configuration
	newConfig := configStr[:phpIndex] + authBlock + "\n\n\t" + configStr[phpIndex:]

	// Write updated config
	if err := os.WriteFile(configFile, []byte(newConfig), 0644); err != nil {
		return fmt.Errorf("failed to write updated config: %v", err)
	}

	// Reload Caddy
	if err := sm.reloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %v", err)
	}

	fmt.Printf("Basic auth added for %s at path %s\n", domain, path)
	return nil
}

// RemoveBasicAuth removes basic authentication from a specific path
func (sm *CaddySiteManager) RemoveBasicAuth(domain, path string) error {
	if sm.Config.Verbose {
		fmt.Printf("Removing basic auth for %s from path %s\n", domain, path)
	}

	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	configFile := filepath.Join(sm.Config.AvailableSites, domain)
	
	// Check if site exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("site '%s' not found", domain)
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would remove basic auth from path: %s\n", path)
		}
		return nil
	}

	// Read current config
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	configStr := string(content)
	
	// Find and remove the auth block
	authPattern := fmt.Sprintf(`\s*# Basic auth for %s.*?}\s*`, regexp.QuoteMeta(path))
	re := regexp.MustCompile(authPattern)
	
	// Also try alternative pattern
	if !re.MatchString(configStr) {
		sanitizedPath := sm.sanitizeName(path)
		authPattern = fmt.Sprintf(`\s*@auth_%s\s*{.*?}\s*basic_auth\s*@auth_%s\s*{.*?}\s*`, 
			regexp.QuoteMeta(sanitizedPath), regexp.QuoteMeta(sanitizedPath))
		re = regexp.MustCompile(authPattern)
	}

	if !re.MatchString(configStr) {
		return fmt.Errorf("basic auth configuration for path %s not found", path)
	}

	newConfig := re.ReplaceAllString(configStr, "")

	// Write updated config
	if err := os.WriteFile(configFile, []byte(newConfig), 0644); err != nil {
		return fmt.Errorf("failed to write updated config: %v", err)
	}

	// Reload Caddy
	if err := sm.reloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %v", err)
	}

	fmt.Printf("Basic auth removed for %s from path %s\n", domain, path)
	return nil
}

// ModifyMaxUpload changes the maximum upload size for a site
func (sm *CaddySiteManager) ModifyMaxUpload(domain, newSize string) error {
	if sm.Config.Verbose {
		fmt.Printf("Modifying max upload size for %s to %s\n", domain, newSize)
	}

	// Validate size format
	if err := sm.validateSizeFormat(newSize); err != nil {
		return fmt.Errorf("invalid size format: %v", err)
	}

	// Get site info
	siteInfo, err := sm.getSiteInfo(domain)
	if err != nil {
		return fmt.Errorf("failed to get site info: %v", err)
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would modify max upload size:\n")
			fmt.Printf("  Domain: %s\n", domain)
			fmt.Printf("  New size: %s\n", newSize)
			fmt.Printf("  PHP-FPM pool: %s\n", siteInfo.PoolName)
		}
		return nil
	}

	// Update PHP-FPM pool configuration
	if err := sm.updatePHPPoolUploadSize(siteInfo, newSize); err != nil {
		return fmt.Errorf("failed to update PHP pool: %v", err)
	}

	// Update Caddy configuration
	if err := sm.updateCaddyUploadSize(domain, newSize); err != nil {
		return fmt.Errorf("failed to update Caddy config: %v", err)
	}

	// Restart PHP-FPM
	if err := sm.restartPHPFPM(siteInfo.PHPVersion); err != nil {
		return fmt.Errorf("failed to restart PHP-FPM: %v", err)
	}

	// Reload Caddy
	if err := sm.reloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %v", err)
	}

	fmt.Printf("Max upload size updated to %s for %s\n", newSize, domain)
	return nil
}

// Helper methods for the modify functionality

// generatePasswordHash generates a bcrypt hash for the password
func (sm *CaddySiteManager) generatePasswordHash(password string) (string, error) {
	// Use Caddy's hash-password command if available
	cmd := exec.Command("caddy", "hash-password", "--plaintext", password)
	output, err := cmd.Output()
	if err != nil {
		// Fallback to basic htpasswd if caddy command fails
		cmd = exec.Command("htpasswd", "-bnB", "temp", password)
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to generate password hash (install caddy or apache2-utils): %v", err)
		}
		// Extract just the hash part from htpasswd output (temp:HASH)
		parts := strings.Split(strings.TrimSpace(string(output)), ":")
		if len(parts) < 2 {
			return "", fmt.Errorf("unexpected htpasswd output format")
		}
		return parts[1], nil
	}
	return strings.TrimSpace(string(output)), nil
}

// sanitizeName creates a safe name for Caddy directives
func (sm *CaddySiteManager) sanitizeName(input string) string {
	// Replace non-alphanumeric characters with underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	return re.ReplaceAllString(input, "_")
}

// validateSizeFormat validates the upload size format
func (sm *CaddySiteManager) validateSizeFormat(size string) error {
	// Allow formats like: 100M, 2G, 2GB, 512MB, 1024K, etc.
	re := regexp.MustCompile(`^[0-9]+[KMGT]B?$`)
	if !re.MatchString(strings.ToUpper(size)) {
		return fmt.Errorf("size must be in format like 100M, 2G, 512MB, etc.")
	}
	return nil
}

// updatePHPPoolUploadSize updates the PHP-FPM pool configuration
func (sm *CaddySiteManager) updatePHPPoolUploadSize(siteInfo *CaddySite, newSize string) error {
	poolConfigFile := fmt.Sprintf("/etc/php/%s/fpm/pool.d/%s.conf", siteInfo.PHPVersion, siteInfo.PoolName)
	
	if _, err := os.Stat(poolConfigFile); os.IsNotExist(err) {
		return fmt.Errorf("PHP pool config file not found: %s", poolConfigFile)
	}

	// Read current config
	content, err := os.ReadFile(poolConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read PHP pool config: %v", err)
	}

	configStr := string(content)
	
	// Update upload_max_filesize and post_max_size
	uploadPattern := regexp.MustCompile(`php_admin_value\[upload_max_filesize\]\s*=\s*[^\n]+`)
	postPattern := regexp.MustCompile(`php_admin_value\[post_max_size\]\s*=\s*[^\n]+`)
	
	configStr = uploadPattern.ReplaceAllString(configStr, fmt.Sprintf("php_admin_value[upload_max_filesize] = %s", newSize))
	configStr = postPattern.ReplaceAllString(configStr, fmt.Sprintf("php_admin_value[post_max_size] = %s", newSize))

	// Write updated config
	if err := os.WriteFile(poolConfigFile, []byte(configStr), 0644); err != nil {
		return fmt.Errorf("failed to write PHP pool config: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Printf("Updated PHP pool configuration: %s\n", poolConfigFile)
	}

	return nil
}

// updateCaddyUploadSize updates the Caddy configuration
func (sm *CaddySiteManager) updateCaddyUploadSize(domain, newSize string) error {
	configFile := filepath.Join(sm.Config.AvailableSites, domain)
	
	// Read current config
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read Caddy config: %v", err)
	}

	configStr := string(content)
	
	// Update request_body max_size
	pattern := regexp.MustCompile(`max_size\s+[^\n]+`)
	if pattern.MatchString(configStr) {
		configStr = pattern.ReplaceAllString(configStr, fmt.Sprintf("max_size %s", newSize))
	} else {
		// If no max_size found, add it to the request_body block
		bodyPattern := regexp.MustCompile(`request_body\s*{`)
		if bodyPattern.MatchString(configStr) {
			configStr = bodyPattern.ReplaceAllString(configStr, fmt.Sprintf("request_body {\n\t\tmax_size %s", newSize))
		} else {
			return fmt.Errorf("could not find request_body configuration in Caddy config")
		}
	}

	// Write updated config
	if err := os.WriteFile(configFile, []byte(configStr), 0644); err != nil {
		return fmt.Errorf("failed to write Caddy config: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Printf("Updated Caddy configuration: %s\n", configFile)
	}

	return nil
}


