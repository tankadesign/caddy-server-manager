package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tankadesign/caddy-site-manager/internal/config"
	"github.com/tankadesign/caddy-site-manager/internal/database"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate existing Caddy configurations to the database",
	Long: `Migrate scans existing Caddy configuration files and converts them to database records.
This is useful when transitioning from the old file-based configuration system to the new
SQLite database system.

The command will:
- Scan all configuration files in available-sites directory (files without extensions)
- Parse domain names and configuration details
- Detect WordPress sites and PHP versions
- Import all configurations into the SQLite database
- Preserve enabled/disabled status based on symlinks in enabled-sites

Examples:
  caddy-site-manager migrate
  caddy-site-manager migrate --dry-run --verbose
  caddy-site-manager migrate --force`,
	RunE: runMigrate,
}

var (
	force      bool
	skipBackup bool
)

func init() {
	rootCmd.AddCommand(migrateCmd)
	
	migrateCmd.Flags().BoolVar(&force, "force", false, "Force migration even if database already contains sites")
	migrateCmd.Flags().BoolVar(&skipBackup, "skip-backup", false, "Skip creating backup of existing database")
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg := config.NewCaddyConfig(viper.GetString("caddy-config"))
	cfg.Verbose = viper.GetBool("verbose")
	cfg.DryRun = viper.GetBool("dry-run")
	
	// Override database path if specified
	if dbPath := viper.GetString("database"); dbPath != "" {
		cfg.DatabasePath = dbPath
	}

	if cfg.Verbose {
		fmt.Printf("Starting migration from Caddy configs to SQLite database...\n")
		fmt.Printf("Caddy config directory: %s\n", cfg.ConfigDir)
		fmt.Printf("Database path: %s\n", cfg.DatabasePath)
		fmt.Printf("Dry run: %t\n", cfg.DryRun)
	}

	// Initialize database connection
	db, err := database.NewDB(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	defer db.Close()

	// Check if database already has sites
	existingSites, err := db.ListSites(nil)
	if err != nil {
		return fmt.Errorf("failed to check existing sites: %v", err)
	}

	if len(existingSites) > 0 && !force {
		fmt.Printf("Database already contains %d site(s). Use --force to proceed anyway.\n", len(existingSites))
		fmt.Println("Existing sites:")
		for _, site := range existingSites {
			status := "disabled"
			if site.IsEnabled {
				status = "enabled"
			}
			fmt.Printf("  - %s (%s)\n", site.Domain, status)
		}
		return nil
	}

	// Create backup if not skipping and not dry run
	if !skipBackup && !cfg.DryRun && len(existingSites) > 0 {
		if err := createDatabaseBackup(cfg.DatabasePath); err != nil {
			return fmt.Errorf("failed to create database backup: %v", err)
		}
	}

	// Scan and migrate configurations
	sites, configFiles, err := scanCaddyConfigs(cfg)
	if err != nil {
		return fmt.Errorf("failed to scan Caddy configs: %v", err)
	}

	if len(sites) == 0 {
		fmt.Println("No Caddy configuration files found to migrate.")
		return nil
	}

	fmt.Printf("Found %d site configuration(s) to migrate:\n", len(sites))
	for _, s := range sites {
		status := "disabled"
		if s.IsEnabled {
			status = "enabled"
		}
		siteType := "PHP"
		if s.IsWordPress {
			siteType = "WordPress"
		}
		fmt.Printf("  - %s (%s, %s, PHP %s)\n", s.Domain, status, siteType, s.PHPVersion)
	}

	if cfg.DryRun {
		fmt.Println("\nDry run mode: No changes will be made to the database.")
		return nil
	}

	// Confirm migration
	if !force {
		fmt.Print("\nProceed with migration? (y/N): ")
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Migration cancelled.")
			return nil
		}
	}

	// Perform the migration
	migrated := 0
	for _, s := range sites {
		if err := db.CreateSite(&s); err != nil {
			fmt.Printf("Failed to migrate %s: %v\n", s.Domain, err)
			continue
		}

		// Extract and migrate basic auth configurations
		basicAuths, err := extractBasicAuthFromConfig(configFiles[s.Domain], s.ID)
		if err != nil {
			fmt.Printf("Warning: Failed to extract basic auth for %s: %v\n", s.Domain, err)
		} else if len(basicAuths) > 0 {
			for _, auth := range basicAuths {
				if err := db.CreateBasicAuth(&auth); err != nil {
					fmt.Printf("Warning: Failed to migrate basic auth for %s%s: %v\n", s.Domain, auth.Path, err)
				} else if cfg.Verbose {
					fmt.Printf("  Migrated basic auth: %s%s (user: %s)\n", s.Domain, auth.Path, auth.Username)
				}
			}
		}

		migrated++
		if cfg.Verbose {
			fmt.Printf("Migrated: %s\n", s.Domain)
		}
	}

	fmt.Printf("\nMigration completed: %d/%d sites migrated successfully.\n", migrated, len(sites))
	
	if migrated > 0 {
		fmt.Println("\nNext steps:")
		fmt.Println("1. Test the migrated configurations with: caddy-site-manager list")
		fmt.Println("2. Verify site functionality")
		fmt.Println("3. Consider backing up your original config files")
	}

	return nil
}

func createDatabaseBackup(dbPath string) error {
	backupPath := dbPath + ".backup." + fmt.Sprintf("%d", os.Getpid())
	
	sourceFile, err := os.Open(dbPath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(backupPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	fmt.Printf("Database backup created: %s\n", backupPath)
	return nil
}

func scanCaddyConfigs(cfg *config.CaddyConfig) ([]database.Site, map[string]string, error) {
	sitesDir := filepath.Join(cfg.ConfigDir, "available-sites")
	enabledDir := filepath.Join(cfg.ConfigDir, "enabled-sites")

	if cfg.Verbose {
		fmt.Printf("Scanning available-sites: %s\n", sitesDir)
		fmt.Printf("Checking enabled-sites: %s\n", enabledDir)
	}

	// Check if directories exist
	if _, err := os.Stat(sitesDir); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("available-sites directory not found: %s", sitesDir)
	}

	// Get all configuration files (files without extensions, which is standard for Caddy)
	var files []string
	
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read sites directory: %v", err)
	}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			fileName := entry.Name()
			filePath := filepath.Join(sitesDir, fileName)
			
			// Skip hidden files and common non-config files
			if strings.HasPrefix(fileName, ".") || 
			   fileName == "README" || fileName == "README.md" ||
			   strings.HasSuffix(fileName, ".txt") ||
			   strings.HasSuffix(fileName, ".log") ||
			   strings.HasSuffix(fileName, ".conf") { // Skip .conf files if any exist
				continue
			}
			
			// Check if file contains Caddy configuration syntax
			if isValidCaddyConfig(filePath) {
				files = append(files, filePath)
			}
		}
	}

	if cfg.Verbose {
		fmt.Printf("Found %d configuration file(s) to examine\n", len(files))
	}

	var sites []database.Site
	configFiles := make(map[string]string) // domain -> config file path
	
	for _, configFile := range files {
		site, err := parseCaddyConfig(configFile, enabledDir, cfg)
		if err != nil {
			if cfg.Verbose {
				fmt.Printf("Warning: Failed to parse %s: %v\n", configFile, err)
			}
			continue
		}
		if site != nil {
			sites = append(sites, *site)
			configFiles[site.Domain] = configFile
		}
	}

	return sites, configFiles, nil
}

func parseCaddyConfig(configFile, enabledDir string, cfg *config.CaddyConfig) (*database.Site, error) {
	if cfg.Verbose {
		fmt.Printf("Parsing: %s\n", configFile)
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	configStr := string(content)
	
	// Extract domain from filename or config content
	domain := extractDomain(configFile, configStr)
	if domain == "" {
		return nil, fmt.Errorf("could not extract domain from config")
	}

	// Check if site is enabled (symlink exists)
	enabledFile := filepath.Join(enabledDir, filepath.Base(configFile))
	isEnabled := false
	if linkInfo, err := os.Lstat(enabledFile); err == nil && linkInfo.Mode()&os.ModeSymlink != 0 {
		if linkTarget, err := os.Readlink(enabledFile); err == nil {
			// Convert relative path to absolute for comparison
			var targetPath string
			if filepath.IsAbs(linkTarget) {
				targetPath = linkTarget
			} else {
				targetPath = filepath.Join(filepath.Dir(enabledFile), linkTarget)
			}
			targetPath = filepath.Clean(targetPath)
			configPath := filepath.Clean(configFile)
			
			if cfg.Verbose {
				fmt.Printf("  Checking symlink: %s -> %s (config: %s)\n", enabledFile, targetPath, configPath)
			}
			
			if targetPath == configPath {
				isEnabled = true
			}
		}
	}

	// Extract document root
	documentRoot := extractDocumentRoot(configStr, domain)
	if documentRoot == "" {
		documentRoot = filepath.Join("/var/www", domain)
	}

	// Detect PHP version
	phpVersion := extractPHPVersion(configStr)
	if phpVersion == "" {
		phpVersion = "8.3" // default
	}

	// Detect if it's WordPress
	isWordPress := detectWordPress(documentRoot, configStr)

	// Extract max upload size
	maxUpload := extractMaxUpload(configStr)
	if maxUpload == "" {
		maxUpload = "256M"
	}

	// Generate pool name
	poolName := generatePoolName(domain)

	// Extract database info for WordPress sites
	var dbName, dbUser, dbPassword string
	if isWordPress {
		dbName, dbUser, dbPassword = extractWordPressDBInfo(documentRoot)
	}

	site := &database.Site{
		Domain:       domain,
		DocumentRoot: documentRoot,
		PHPVersion:   phpVersion,
		IsWordPress:  isWordPress,
		IsEnabled:    isEnabled,
		MaxUpload:    maxUpload,
		DBName:       dbName,
		DBUser:       dbUser,
		DBPassword:   dbPassword,
		PoolName:     poolName,
	}

	return site, nil
}

func extractDomain(configFile, content string) string {
	// First try to extract from filename (standard Caddy approach)
	filename := filepath.Base(configFile)
	if isValidDomain(filename) {
		return filename
	}

	// Try to extract from config content (look for domain at start of line)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		
		// Look for domain pattern at start of block
		if strings.Contains(line, "{") {
			domain := strings.TrimSpace(strings.Split(line, "{")[0])
			if isValidDomain(domain) {
				return domain
			}
		}
	}

	return ""
}

func extractDocumentRoot(content, domain string) string {
	// Look for root directive with * wildcard pattern: root * /path/to/site
	re := regexp.MustCompile(`root\s+\*\s+([^\s\n]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.Trim(matches[1], `"'`)
	}
	
	// Fallback: Look for simple root directive: root /path/to/site
	re = regexp.MustCompile(`root\s+([^\s\n*]+)`)
	matches = re.FindStringSubmatch(content)
	if len(matches) > 1 {
		path := strings.Trim(matches[1], `"'`)
		// Skip if it's just the wildcard
		if path != "*" {
			return path
		}
	}
	
	return ""
}

func extractPHPVersion(content string) string {
	// Look for PHP-FPM socket path or version reference
	re := regexp.MustCompile(`php(\d+\.\d+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}

	// Look for common PHP versions in fastcgi paths
	versions := []string{"8.3", "8.2", "8.1", "8.0", "7.4"}
	for _, version := range versions {
		if strings.Contains(content, version) {
			return version
		}
	}

	return ""
}

func extractMaxUpload(content string) string {
	// Look for client_max_body_size equivalent or similar
	re := regexp.MustCompile(`max_body_size\s+([^\s\n]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.Trim(matches[1], `"'`)
	}
	return ""
}

func detectWordPress(documentRoot, content string) bool {
	// Check for wp-config.php in document root
	wpConfigPath := filepath.Join(documentRoot, "wp-config.php")
	if _, err := os.Stat(wpConfigPath); err == nil {
		return true
	}

	// Check for WordPress-specific patterns in config
	wpPatterns := []string{
		"wp-admin",
		"wp-content",
		"wp-includes",
		"wordpress",
		"wp-config",
	}

	contentLower := strings.ToLower(content)
	for _, pattern := range wpPatterns {
		if strings.Contains(contentLower, pattern) {
			return true
		}
	}

	return false
}

func extractWordPressDBInfo(documentRoot string) (string, string, string) {
	wpConfigPath := filepath.Join(documentRoot, "wp-config.php")
	content, err := os.ReadFile(wpConfigPath)
	if err != nil {
		return "", "", ""
	}

	contentStr := string(content)
	
	dbName := extractWPDefine(contentStr, "DB_NAME")
	dbUser := extractWPDefine(contentStr, "DB_USER")
	dbPassword := extractWPDefine(contentStr, "DB_PASSWORD")

	return dbName, dbUser, dbPassword
}

func extractWPDefine(content, defineName string) string {
	patterns := []string{
		fmt.Sprintf(`define\s*\(\s*['"]%s['"]\s*,\s*['"]([^'"]+)['"]`, defineName),
		fmt.Sprintf(`define\s*\(\s*'%s'\s*,\s*'([^']+)'`, defineName),
		fmt.Sprintf(`define\s*\(\s*"%s"\s*,\s*"([^"]+)"`, defineName),
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

func isValidDomain(domain string) bool {
	// Basic domain validation
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}
	
	// Check for valid characters
	re := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-\.]*[a-zA-Z0-9]$`)
	return re.MatchString(domain)
}

func generatePoolName(domain string) string {
	// Convert domain to valid pool name (alphanumeric + underscore)
	poolName := strings.ReplaceAll(domain, ".", "_")
	poolName = strings.ReplaceAll(poolName, "-", "_")
	return poolName
}

// isValidCaddyConfig checks if a file contains valid Caddy configuration syntax
func isValidCaddyConfig(filePath string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	
	contentStr := string(content)
	
	// Check for Caddy-specific patterns
	caddyPatterns := []string{
		"{",          // Caddy uses curly braces for blocks
		"root *",     // Common Caddy directive
		"php_fastcgi", // PHP processing
		"file_server", // Static file serving
		"encode gzip", // Compression
		"try_files",   // URL rewriting
		"redir",       // Redirects
		"route",       // Route handling
		"handle",      // Request handling
	}
	
	contentLower := strings.ToLower(contentStr)
	matchCount := 0
	
	for _, pattern := range caddyPatterns {
		if strings.Contains(contentLower, strings.ToLower(pattern)) {
			matchCount++
		}
	}
	
	// If we find at least 2 Caddy-specific patterns, it's likely a Caddy config
	return matchCount >= 2
}

// extractBasicAuthFromConfig extracts basic auth configurations from a Caddy config file
func extractBasicAuthFromConfig(configFilePath string, siteID int) ([]database.BasicAuth, error) {
	content, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	contentStr := string(content)
	var basicAuths []database.BasicAuth

	// Look for basic_auth blocks in different contexts
	// Pattern 1: Route-based basic auth like "route /path* { basic_auth { ... } }"
	routePattern := regexp.MustCompile(`route\s+([^\s{]+)[^{]*\{[^}]*basic_auth\s*\{([^}]+)\}`)
	routeMatches := routePattern.FindAllStringSubmatch(contentStr, -1)
	
	for _, match := range routeMatches {
		if len(match) >= 3 {
			path := strings.TrimSpace(match[1])
			// Remove trailing * from path patterns
			path = strings.TrimSuffix(path, "*")
			authBlock := match[2]
			
			auths := parseBasicAuthBlock(authBlock, path, siteID)
			basicAuths = append(basicAuths, auths...)
		}
	}

	// Pattern 2: Direct basic_auth blocks within site config (not inside route blocks)
	// Look for basic_auth that is not preceded by "route" on the same logical level
	lines := strings.Split(contentStr, "\n")
	inRoute := false
	routeBraceCount := 0
	
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		
		// Track if we're inside a route block
		if strings.HasPrefix(trimmedLine, "route ") {
			inRoute = true
			routeBraceCount = 0
		}
		
		// Count braces to track nesting within route
		if inRoute {
			routeBraceCount += strings.Count(line, "{")
			routeBraceCount -= strings.Count(line, "}")
			
			// If we've closed all braces, we're out of the route
			if routeBraceCount <= 0 {
				inRoute = false
			}
		}
		
		// Only process basic_auth blocks that are NOT inside route blocks
		if !inRoute && strings.Contains(trimmedLine, "basic_auth {") {
			// Extract the basic_auth block content
			authContent := ""
			braceCount := 1
			j := i + 1
			
			for j < len(lines) && braceCount > 0 {
				authContent += lines[j] + "\n"
				braceCount += strings.Count(lines[j], "{")
				braceCount -= strings.Count(lines[j], "}")
				j++
			}
			
			if braceCount == 0 {
				// For direct basic auth, assume it applies to root path
				auths := parseBasicAuthBlock(authContent, "/", siteID)
				basicAuths = append(basicAuths, auths...)
			}
		}
	}

	return basicAuths, nil
}

// parseBasicAuthBlock parses a basic_auth block and extracts username/password pairs
func parseBasicAuthBlock(authBlock, path string, siteID int) []database.BasicAuth {
	var basicAuths []database.BasicAuth
	
	// Parse username password pairs
	// Format: username hashedpassword
	lines := strings.Split(authBlock, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Split by whitespace to get username and password
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			username := parts[0]
			password := parts[1]
			
			basicAuth := database.BasicAuth{
				SiteID:   siteID,
				Path:     path,
				Username: username,
				Password: password,
			}
			basicAuths = append(basicAuths, basicAuth)
		}
	}
	
	return basicAuths
}
