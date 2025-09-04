package site

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/tankadesign/caddy-site-manager/internal/database"
)

// Utility functions

// generatePoolName generates a PHP-FPM pool name from domain
func generatePoolName(domain string) string {
	// Convert domain to valid pool name (alphanumeric + underscore)
	poolName := strings.ReplaceAll(domain, ".", "_")
	poolName = strings.ReplaceAll(poolName, "-", "_")
	return poolName
}

// generateDBName generates a database name from domain
func generateDBName(domain string) string {
	// Convert domain to valid database name (alphanumeric + underscore)
	dbName := strings.ReplaceAll(domain, ".", "_")
	dbName = strings.ReplaceAll(dbName, "-", "_")
	// Ensure it doesn't start with a number
	if len(dbName) > 0 && dbName[0] >= '0' && dbName[0] <= '9' {
		dbName = "wp_" + dbName
	}
	return dbName
}

// generateRandomPassword generates a random password
func generateRandomPassword() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:16], nil
}

// confirmDeletion prompts the user for confirmation
func confirmDeletion() bool {
	fmt.Print("Are you sure you want to proceed? (y/N): ")
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// SQLite operations for the SQLiteSiteManager

// initTemplates initializes the configuration templates
func (sm *SQLiteSiteManager) initTemplates() error {
	// PHP-FPM pool template
	phpPoolTemplate := `[{{.PoolName}}]
user = www-data
group = www-data
listen = /run/php/php{{.PHPVersion}}-fpm-{{.PoolName}}.sock
listen.owner = www-data
listen.group = www-data
listen.mode = 0660

; Process manager settings optimized for PHP
pm = dynamic
pm.max_children = 10
pm.start_servers = 3
pm.min_spare_servers = 2
pm.max_spare_servers = 5
pm.max_requests = 1000

; PHP settings with configurable upload size
php_admin_value[upload_max_filesize] = {{.MaxUpload}}
php_admin_value[post_max_size] = {{.MaxUpload}}
php_admin_value[max_execution_time] = 300
php_admin_value[max_input_time] = 300
php_admin_value[memory_limit] = 512M
php_admin_value[max_file_uploads] = 50

; General PHP optimizations
php_admin_value[max_input_vars] = 5000
php_admin_value[max_input_nesting_level] = 64

; Security settings
php_admin_flag[allow_url_fopen] = on
php_admin_flag[allow_url_include] = off
php_admin_flag[expose_php] = off

; Error handling
php_admin_flag[display_errors] = off
php_admin_flag[log_errors] = on
php_admin_value[error_log] = /var/log/php/{{.PoolName}}-error.log

; Session settings
php_admin_value[session.save_path] = /var/lib/php/sessions
php_admin_flag[session.cookie_httponly] = on

; OPcache settings for better performance
php_admin_flag[opcache.enable] = on
php_admin_value[opcache.memory_consumption] = 128
php_admin_value[opcache.interned_strings_buffer] = 8
php_admin_value[opcache.max_accelerated_files] = 4000
php_admin_flag[opcache.validate_timestamps] = on
php_admin_value[opcache.revalidate_freq] = 60
`

	// Caddy configuration template for basic PHP sites
	caddyTemplate := `# PHP site: {{.Domain}} (Custom PHP-FPM Pool: {{.PoolName}})
{{.Domain}} {
	root * {{.DocumentRoot}}
	encode gzip

	# Set request body limit to match PHP settings
	request_body {
		max_size {{.MaxUpload}}
	}

	# Enable clean URLs for PHP files (removes .php extension requirement)
	try_files {path} {path}.php

	# PHP processing using custom PHP pool
	php_fastcgi unix//run/php/php{{.PHPVersion}}-fpm-{{.PoolName}}.sock {
		index index.php
	}

	# Security headers
	header {
		# Remove server info
		-Server
		
		# Security headers
		X-Content-Type-Options nosniff
		X-XSS-Protection "1; mode=block"
		Referrer-Policy strict-origin-when-cross-origin
	}

	# File server for static files
	file_server
}

www.{{.Domain}} {
	redir https://{{.Domain}}{uri}
}
`

	// WordPress specific template
	wpTemplate := `# WordPress site: {{.Domain}} (Custom PHP-FPM Pool: {{.PoolName}})
{{.Domain}} {
	root * {{.DocumentRoot}}
	encode gzip

	# Set request body limit to match PHP settings
	request_body {
		max_size {{.MaxUpload}}
	}

	# PHP processing using custom PHP pool
	php_fastcgi unix//run/php/php{{.PHPVersion}}-fpm-{{.PoolName}}.sock {
		index index.php
	}

	# WordPress pretty permalinks
	try_files {path} {path}/ /index.php?{query}

	# Deny access to sensitive WordPress files
	@forbidden {
		path *.sql
		path /wp-config.php
		path /wp-content/debug.log
		path /.htaccess
		path /wp-content/uploads/*.php
	}
	respond @forbidden 403

	# Security headers
	header {
		# Remove server info
		-Server
		
		# Security headers
		X-Content-Type-Options nosniff
		X-XSS-Protection "1; mode=block"
		Referrer-Policy strict-origin-when-cross-origin
	}

	# File server for static files
	file_server
}

www.{{.Domain}} {
	redir https://{{.Domain}}{uri}
}
`

	var err error
	sm.phpPoolTmpl, err = template.New("phppool").Parse(phpPoolTemplate)
	if err != nil {
		return err
	}

	sm.caddyTmpl, err = template.New("caddy").Parse(caddyTemplate)
	if err != nil {
		return err
	}

	sm.wpTmpl, err = template.New("wordpress").Parse(wpTemplate)
	if err != nil {
		return err
	}

	return nil
}

// checkPhysicalConflicts checks for existing file system conflicts
func (sm *SQLiteSiteManager) checkPhysicalConflicts(site *database.Site) error {
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

	configFile := filepath.Join(sm.Config.AvailableSites, site.Domain)
	
	// Check if config file already exists
	if _, err := os.Stat(configFile); err == nil {
		if !sm.Config.DryRun {
			if !sm.confirmOverwrite(fmt.Sprintf("Domain configuration '%s' already exists", site.Domain)) {
				return fmt.Errorf("aborting site setup")
			}
			if sm.Config.Verbose {
				fmt.Println("Removing existing configuration...")
			}
			// Remove both config and symlink
			os.Remove(configFile)
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
func (sm *SQLiteSiteManager) checkDatabaseConflicts(site *database.Site) error {
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
func (sm *SQLiteSiteManager) createPHPFPMPool(site *database.Site) error {
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
	if err := exec.Command("chown", "ubuntu:www-data", "/var/log/php").Run(); err != nil {
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
func (sm *SQLiteSiteManager) restartPHPFPM(phpVersion string) error {
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
func (sm *SQLiteSiteManager) createSiteDirectory(site *database.Site) error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would create site directory: %s\n", site.DocumentRoot)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Println("Creating site directory...")
	}

	if err := os.MkdirAll(site.DocumentRoot, 0775); err != nil {
		return fmt.Errorf("failed to create site directory: %v", err)
	}

	return nil
}

// createBasicPHPSite creates a basic PHP site structure
func (sm *SQLiteSiteManager) createBasicPHPSite(site *database.Site) error {
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
func (sm *SQLiteSiteManager) createWordPressSite(site *database.Site) error {
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

// setPermissions sets proper file permissions for the site
func (sm *SQLiteSiteManager) setPermissions(site *database.Site) error {
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
	if err := exec.Command("chown", "-R", "ubuntu:www-data", site.DocumentRoot).Run(); err != nil {
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
func (sm *SQLiteSiteManager) generateCaddyConfig(site *database.Site, configFile string) error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would create Caddy config: %s\n", configFile)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Printf("Creating Caddy configuration for %s...\n", site.Domain)
	}

	file, err := os.Create(configFile)
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

// regenerateCaddyConfig regenerates the complete Caddy configuration including basic auth
func (sm *SQLiteSiteManager) regenerateCaddyConfig(siteID int, configFile string) error {
	// First, get the site from database by finding it with the ID
	// Since we need the domain, we'll get all sites and find the matching one
	allSites, err := sm.DB.ListSites(nil)
	if err != nil {
		return fmt.Errorf("failed to list sites: %v", err)
	}

	var targetSite *database.Site
	for _, site := range allSites {
		if site.ID == siteID {
			targetSite = &site
			break
		}
	}

	if targetSite == nil {
		return fmt.Errorf("site with ID %d not found", siteID)
	}

	// Get site with auth from database
	siteWithAuth, err := sm.DB.GetSiteWithAuth(targetSite.Domain)
	if err != nil {
		return fmt.Errorf("failed to get site with auth: %v", err)
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would regenerate Caddy config: %s\n", configFile)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Printf("Regenerating Caddy configuration for %s...\n", siteWithAuth.Domain)
	}

	// Start with base template
	var baseConfig strings.Builder
	var tmpl *template.Template
	if siteWithAuth.IsWordPress {
		tmpl = sm.wpTmpl
	} else {
		tmpl = sm.caddyTmpl
	}

	if err := tmpl.Execute(&baseConfig, &siteWithAuth.Site); err != nil {
		return fmt.Errorf("failed to execute base template: %v", err)
	}

	config := baseConfig.String()

	// Add basic auth blocks if any exist
	if len(siteWithAuth.BasicAuths) > 0 {
		config = sm.addBasicAuthToConfig(config, siteWithAuth.BasicAuths)
	}

	// Write the complete config
	if err := os.WriteFile(configFile, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// addBasicAuthToConfig adds basic auth blocks to the Caddy configuration using route syntax
func (sm *SQLiteSiteManager) addBasicAuthToConfig(config string, auths []database.BasicAuth) string {
	// Group auths by path
	authsByPath := make(map[string][]database.BasicAuth)
	for _, auth := range auths {
		authsByPath[auth.Path] = append(authsByPath[auth.Path], auth)
	}

	// Find the insertion point (before try_files or PHP processing)
	insertIndex := strings.Index(config, "try_files")
	if insertIndex == -1 {
		insertIndex = strings.Index(config, "php_fastcgi")
	}
	if insertIndex == -1 {
		// Fallback: insert before file_server
		insertIndex = strings.Index(config, "file_server")
	}
	if insertIndex == -1 {
		// Last resort: insert before the closing brace
		insertIndex = strings.LastIndex(config, "}")
	}

	var authBlocks strings.Builder

	// Generate route blocks for each path
	for path, pathAuths := range authsByPath {
		// Use proper path pattern for routes
		pathPattern := path
		if !strings.HasSuffix(pathPattern, "*") {
			pathPattern += "*"
		}
		
		authBlocks.WriteString(fmt.Sprintf(`
	route %s {
		basic_auth {`, pathPattern))

		// Add all users for this path
		for _, auth := range pathAuths {
			authBlocks.WriteString(fmt.Sprintf(`
			%s %s`, auth.Username, auth.Password))
		}

		authBlocks.WriteString(`
		}
	}`)
	}

	// Insert auth blocks before the identified insertion point
	return config[:insertIndex] + authBlocks.String() + "\n\t" + config[insertIndex:]
}

// validateAndReloadCaddy validates and reloads the Caddy configuration
func (sm *SQLiteSiteManager) validateAndReloadCaddy() error {
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
func (sm *SQLiteSiteManager) reloadCaddy() error {
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

// printSuccessMessage prints the success message after site creation
func (sm *SQLiteSiteManager) printSuccessMessage(site *database.Site) {
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
	configFile := filepath.Join(sm.Config.AvailableSites, site.Domain)
	fmt.Printf("Configuration: %s\n", configFile)
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

// hardDelete performs complete removal
func (sm *SQLiteSiteManager) hardDelete(site *database.Site, opts *SiteDeleteOptions) error {
	// Show warning and confirm
	if !opts.Force && !sm.Config.DryRun {
		fmt.Printf("WARNING: This will permanently delete:\n")
		fmt.Printf("  - Domain: %s%s\n", opts.Domain, 
			map[bool]string{true: " (WordPress)", false: ""}[site.IsWordPress])
		fmt.Printf("  - Directory: %s\n", site.DocumentRoot)
		if site.IsWordPress {
			fmt.Printf("  - Associated database and user\n")
		}
		fmt.Printf("  - Config file from available-sites\n")
		fmt.Printf("  - Symlink from enabled-sites\n")
		fmt.Printf("  - Custom PHP-FPM pool: %s (if exists)\n", site.PoolName)
		fmt.Printf("\n")

		if !confirmDeletion() {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	if sm.Config.Verbose {
		fmt.Printf("Starting complete deletion process for %s...\n", opts.Domain)
	}

	// Delete database first (if WordPress)
	if site.IsWordPress {
		if err := sm.deleteDatabase(site); err != nil {
			return fmt.Errorf("failed to delete database: %v", err)
		}
	}

	// Remove PHP-FPM pool
	if err := sm.removePHPFPMPool(site); err != nil {
		return fmt.Errorf("failed to remove PHP-FPM pool: %v", err)
	}

	// Remove symlink
	symlinkPath := filepath.Join(sm.Config.EnabledSites, opts.Domain)
	if err := sm.removeSymlink(symlinkPath); err != nil {
		return err
	}

	// Delete config file
	configFile := filepath.Join(sm.Config.AvailableSites, opts.Domain)
	if err := sm.removeFile(configFile, "config file"); err != nil {
		return err
	}

	// Delete from database
	if err := sm.DB.DeleteSite(opts.Domain); err != nil {
		return fmt.Errorf("failed to delete site from database: %v", err)
	}

	// Reload Caddy
	if err := sm.reloadCaddy(); err != nil {
		return err
	}

	// Delete web directory last
	if err := sm.removeDirectory(site.DocumentRoot); err != nil {
		return err
	}

	fmt.Printf("Site '%s' has been completely deleted.\n", opts.Domain)
	return nil
}

// softDelete removes only the symlink (disables the site)
func (sm *SQLiteSiteManager) softDelete(site *database.Site, opts *SiteDeleteOptions) error {
	if sm.Config.Verbose {
		fmt.Printf("Performing soft delete for %s (removing symlink only)...\n", opts.Domain)
	}

	// Update database to mark as disabled
	site.IsEnabled = false
	if err := sm.DB.UpdateSite(site); err != nil {
		return fmt.Errorf("failed to update site status in database: %v", err)
	}

	symlinkPath := filepath.Join(sm.Config.EnabledSites, opts.Domain)
	
	if err := sm.removeSymlink(symlinkPath); err != nil {
		return err
	}

	if err := sm.reloadCaddy(); err != nil {
		return err
	}

	fmt.Printf("Site '%s' has been disabled (symlink removed).\n", opts.Domain)
	fmt.Printf("To completely delete the site, run with --hard flag\n")
	
	return nil
}

// Helper functions for database operations and other utilities

// Database helper methods
func (sm *SQLiteSiteManager) databaseExists(dbName string) (bool, error) {
	cmd := exec.Command("mysql", "-u", "root", "-e", fmt.Sprintf("SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = '%s'", dbName))
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(output), dbName), nil
}

func (sm *SQLiteSiteManager) databaseUserExists(dbUser string) (bool, error) {
	cmd := exec.Command("mysql", "-u", "root", "-e", fmt.Sprintf("SELECT User FROM mysql.user WHERE User = '%s'", dbUser))
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(output), dbUser), nil
}

func (sm *SQLiteSiteManager) dropDatabase(dbName string) error {
	cmd := exec.Command("mysql", "-u", "root", "-e", fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
	return cmd.Run()
}

func (sm *SQLiteSiteManager) dropDatabaseUser(dbUser string) error {
	cmd := exec.Command("mysql", "-u", "root", "-e", fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost'", dbUser))
	return cmd.Run()
}

func (sm *SQLiteSiteManager) deleteDatabase(site *database.Site) error {
	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would delete database and user: %s\n", site.DBName)
		}
		return nil
	}

	if sm.Config.Verbose {
		fmt.Printf("Deleting database '%s' and user '%s'...\n", site.DBName, site.DBUser)
	}

	queries := []string{
		fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", site.DBName),
		fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost'", site.DBUser),
		"FLUSH PRIVILEGES",
	}

	for _, query := range queries {
		cmd := exec.Command("mysql", "-u", "root", "-e", query)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to execute database query: %v", err)
		}
	}

	if sm.Config.Verbose {
		fmt.Println("Database and user deleted successfully")
	}

	return nil
}

func (sm *SQLiteSiteManager) setupWordPressDatabase(site *database.Site) error {
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

func (sm *SQLiteSiteManager) generateWordPressConfig(site *database.Site) error {
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

func (sm *SQLiteSiteManager) getWordPressSalts() (string, error) {
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

func (sm *SQLiteSiteManager) copyDir(src, dst string) error {
	return exec.Command("cp", "-R", src+"/.", dst+"/").Run()
}

func (sm *SQLiteSiteManager) confirmOverwrite(message string) bool {
	fmt.Printf("Warning: %s.\n", message)
	fmt.Print("Do you want to overwrite? (y/n): ")
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

func (sm *SQLiteSiteManager) removePHPFPMPool(site *database.Site) error {
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

func (sm *SQLiteSiteManager) removeSymlink(symlinkPath string) error {
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

func (sm *SQLiteSiteManager) removeFile(filePath, description string) error {
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

func (sm *SQLiteSiteManager) removeDirectory(dirPath string) error {
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

// Modify functionality helper methods

func (sm *SQLiteSiteManager) generatePasswordHash(password string) (string, error) {
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

func (sm *SQLiteSiteManager) sanitizeName(input string) string {
	// Replace non-alphanumeric characters with underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	return re.ReplaceAllString(input, "_")
}

func (sm *SQLiteSiteManager) validateSizeFormat(size string) error {
	// Allow formats like: 100M, 2G, 2GB, 512MB, 1024K, etc.
	re := regexp.MustCompile(`^[0-9]+[KMGT]B?$`)
	if !re.MatchString(strings.ToUpper(size)) {
		return fmt.Errorf("size must be in format like 100M, 2G, 512MB, etc.")
	}
	return nil
}

func (sm *SQLiteSiteManager) updatePHPPoolUploadSize(site *database.Site, newSize string) error {
	poolConfigFile := fmt.Sprintf("/etc/php/%s/fpm/pool.d/%s.conf", site.PHPVersion, site.PoolName)
	
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
