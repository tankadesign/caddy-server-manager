package site

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/falcon/caddy-site-manager/internal/config"
)

// SiteCreateOptions represents options for creating a site
type SiteCreateOptions struct {
	Domain     string
	WordPress  bool
	DBName     string
	DBPassword string
	MaxUpload  string
	PHPVersion string
}

// SiteDeleteOptions represents options for deleting a site
type SiteDeleteOptions struct {
	Domain     string
	Hard       bool
	Force      bool
}

// CaddySite represents a website configuration
type CaddySite struct {
	Domain        string
	Path          string
	PHPVersion    string
	IsWordPress   bool
	IsEnabled     bool
	ConfigFile    string
	DocumentRoot  string
	PoolName      string
	DBName        string
	DBUser        string
	DBPassword    string
	MaxUpload     string
}

// CaddySiteManager handles site operations
type CaddySiteManager struct {
	Config         *config.CaddyConfig
	caddyTmpl      *template.Template
	wpTmpl         *template.Template
	phpPoolTmpl    *template.Template
}

// NewCaddySiteManager creates a new SiteManager
func NewCaddySiteManager(cfg *config.CaddyConfig) (*CaddySiteManager, error) {
	sm := &CaddySiteManager{
		Config: cfg,
	}

	// Initialize templates
	if err := sm.initTemplates(); err != nil {
		return nil, fmt.Errorf("failed to initialize templates: %v", err)
	}

	return sm, nil
}

// CreateSite creates a new site with all the comprehensive features from the bash script
func (sm *CaddySiteManager) CreateSite(opts *SiteCreateOptions) error {
	// Validate domain
	if opts.Domain == "" {
		return fmt.Errorf("domain is required")
	}

	// Auto-generate pool name
	poolName := generatePoolName(opts.Domain)
	
	// Set defaults
	if opts.PHPVersion == "" {
		opts.PHPVersion = sm.Config.PHPVersion
	}
	if opts.MaxUpload == "" {
		opts.MaxUpload = "256M"
	}

	// Auto-generate database credentials if WordPress is enabled
	var dbName, dbUser, dbPassword string
	if opts.WordPress {
		if opts.DBName == "" {
			dbName = generateDBName(opts.Domain)
		} else {
			dbName = opts.DBName
		}
		if opts.DBPassword == "" {
			var err error
			dbPassword, err = generateRandomPassword()
			if err != nil {
				return fmt.Errorf("failed to generate database password: %v", err)
			}
		} else {
			dbPassword = opts.DBPassword
		}
		dbUser = dbName // Set DB_USER to same as DB_NAME as per requirement
	}

	site := &CaddySite{
		Domain:       opts.Domain,
		Path:         opts.Domain,
		PHPVersion:   opts.PHPVersion,
		IsWordPress:  opts.WordPress,
		ConfigFile:   filepath.Join(sm.Config.AvailableSites, opts.Domain),
		DocumentRoot: filepath.Join("/var/www/sites", opts.Domain),
		PoolName:     poolName,
		DBName:       dbName,
		DBUser:       dbUser,
		DBPassword:   dbPassword,
		MaxUpload:    opts.MaxUpload,
	}

	if sm.Config.Verbose {
		fmt.Printf("Setting up %s site for domain: %s\n", 
			map[bool]string{true: "WordPress", false: "PHP"}[opts.WordPress], 
			opts.Domain)
		if opts.WordPress {
			fmt.Printf("Database name: %s\n", dbName)
			fmt.Printf("Database user: %s\n", dbUser)
		}
		fmt.Printf("PHP-FPM Pool: %s\n", poolName)
		fmt.Printf("Max upload size: %s\n", opts.MaxUpload)
	}

	// Check for conflicts
	if err := sm.checkConflicts(site); err != nil {
		return err
	}

	// Create custom PHP-FPM pool
	if err := sm.createPHPFPMPool(site); err != nil {
		return fmt.Errorf("failed to create PHP-FPM pool: %v", err)
	}

	// Restart PHP-FPM
	if err := sm.restartPHPFPM(site.PHPVersion); err != nil {
		return fmt.Errorf("failed to restart PHP-FPM: %v", err)
	}

	// Create site directory
	if err := sm.createSiteDirectory(site); err != nil {
		return fmt.Errorf("failed to create site directory: %v", err)
	}

	// Create site content
	if site.IsWordPress {
		if err := sm.createWordPressSite(site); err != nil {
			return fmt.Errorf("failed to create WordPress site: %v", err)
		}
	} else {
		if err := sm.createBasicPHPSite(site); err != nil {
			return fmt.Errorf("failed to create basic PHP site: %v", err)
		}
	}

	// Set permissions
	if err := sm.setPermissions(site); err != nil {
		return fmt.Errorf("failed to set permissions: %v", err)
	}

	// Generate Caddy configuration
	if err := sm.generateCaddyConfig(site); err != nil {
		return fmt.Errorf("failed to generate Caddy config: %v", err)
	}

	// Enable the site
	if err := sm.EnableSite(opts.Domain); err != nil {
		return fmt.Errorf("failed to enable site: %v", err)
	}

	// Validate and reload Caddy
	if err := sm.validateAndReloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %v", err)
	}

	// Print success message
	sm.printSuccessMessage(site)

	return nil
}

// DeleteSite deletes a site based on the delete options
func (sm *CaddySiteManager) DeleteSite(opts *SiteDeleteOptions) error {
	if opts.Domain == "" {
		return fmt.Errorf("domain is required")
	}

	// Check if site exists
	configFile := filepath.Join(sm.Config.AvailableSites, opts.Domain)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("site '%s' not found", opts.Domain)
	}

	if opts.Hard {
		return sm.hardDelete(opts)
	}
	return sm.softDelete(opts)
}

// softDelete removes only the symlink (disables the site)
func (sm *CaddySiteManager) softDelete(opts *SiteDeleteOptions) error {
	if sm.Config.Verbose {
		fmt.Printf("Performing soft delete for %s (removing symlink only)...\n", opts.Domain)
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

// hardDelete performs complete removal
func (sm *CaddySiteManager) hardDelete(opts *SiteDeleteOptions) error {
	// Get site info
	site, err := sm.getSiteInfo(opts.Domain)
	if err != nil {
		return err
	}

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

// Helper methods

func generatePoolName(domain string) string {
	// Replace non-alphanumeric chars with underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	return re.ReplaceAllString(domain, "_")
}

func generateDBName(domain string) string {
	return generatePoolName(domain)
}

func generateRandomPassword() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

func confirmDeletion() bool {
	fmt.Print("Are you absolutely sure you want to proceed? Type 'DELETE' to confirm: ")
	var confirmation string
	fmt.Scanln(&confirmation)
	return confirmation == "DELETE"
}

// initTemplates initializes the configuration templates
func (sm *CaddySiteManager) initTemplates() error {
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
