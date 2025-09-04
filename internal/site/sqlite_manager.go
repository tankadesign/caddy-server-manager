package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/tankadesign/caddy-site-manager/internal/config"
	"github.com/tankadesign/caddy-site-manager/internal/database"
)

// SQLiteSiteManager handles site operations using SQLite database
type SQLiteSiteManager struct {
	Config      *config.CaddyConfig
	DB          *database.DB
	caddyTmpl   *template.Template
	wpTmpl      *template.Template
	phpPoolTmpl *template.Template
}

// NewSQLiteSiteManager creates a new SQLite-based site manager
func NewSQLiteSiteManager(cfg *config.CaddyConfig, db *database.DB) (*SQLiteSiteManager, error) {
	sm := &SQLiteSiteManager{
		Config: cfg,
		DB:     db,
	}

	// Initialize templates
	if err := sm.initTemplates(); err != nil {
		return nil, fmt.Errorf("failed to initialize templates: %v", err)
	}

	return sm, nil
}

// CreateSite creates a new site using SQLite database
func (sm *SQLiteSiteManager) CreateSite(opts *SiteCreateOptions) error {
	// Validate domain
	if opts.Domain == "" {
		return fmt.Errorf("domain is required")
	}

	// Check if site already exists
	exists, err := sm.DB.SiteExists(opts.Domain)
	if err != nil {
		return fmt.Errorf("failed to check site existence: %v", err)
	}
	if exists {
		return fmt.Errorf("site '%s' already exists", opts.Domain)
	}

	// Set defaults
	if opts.PHPVersion == "" {
		opts.PHPVersion = sm.Config.PHPVersion
	}
	if opts.MaxUpload == "" {
		opts.MaxUpload = "256M"
	}

	// Auto-generate pool name
	poolName := generatePoolName(opts.Domain)
	
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

	// Create site record
	site := &database.Site{
		Domain:       opts.Domain,
		DocumentRoot: filepath.Join("/var/www/sites", opts.Domain),
		PHPVersion:   opts.PHPVersion,
		IsWordPress:  opts.WordPress,
		IsEnabled:    false, // Will be enabled after successful creation
		MaxUpload:    opts.MaxUpload,
		DBName:       dbName,
		DBUser:       dbUser,
		DBPassword:   dbPassword,
		PoolName:     poolName,
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

	// Check for conflicts (directories, files)
	if err := sm.checkPhysicalConflicts(site); err != nil {
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
	configFile := filepath.Join(sm.Config.AvailableSites, opts.Domain)
	if err := sm.generateCaddyConfig(site, configFile); err != nil {
		return fmt.Errorf("failed to generate Caddy config: %v", err)
	}

	// Store site in database
	if err := sm.DB.CreateSite(site); err != nil {
		return fmt.Errorf("failed to store site in database: %v", err)
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

// DeleteSite deletes a site using SQLite database
func (sm *SQLiteSiteManager) DeleteSite(opts *SiteDeleteOptions) error {
	if opts.Domain == "" {
		return fmt.Errorf("domain is required")
	}

	// Get site from database
	site, err := sm.DB.GetSite(opts.Domain)
	if err != nil {
		return err
	}

	if opts.Hard {
		return sm.hardDelete(site, opts)
	}
	return sm.softDelete(site, opts)
}

// EnableSite enables a site by creating a symlink and updating database
func (sm *SQLiteSiteManager) EnableSite(domain string) error {
	if sm.Config.Verbose {
		fmt.Printf("Enabling site: %s\n", domain)
	}

	// Get site from database
	site, err := sm.DB.GetSite(domain)
	if err != nil {
		return err
	}

	if site.IsEnabled && !sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Site %s is already enabled\n", domain)
		}
		return nil
	}

	configFile := filepath.Join(sm.Config.AvailableSites, domain)
	symlinkPath := filepath.Join(sm.Config.EnabledSites, domain)

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would create symlink: %s -> %s\n", symlinkPath, configFile)
			fmt.Printf("Would update database to set site as enabled\n")
		}
		return nil
	}

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("site configuration not found: %s", domain)
	}

	// Create symlink
	if err := os.Symlink(configFile, symlinkPath); err != nil {
		return fmt.Errorf("failed to create symlink: %v", err)
	}

	// Update database
	site.IsEnabled = true
	if err := sm.DB.UpdateSite(site); err != nil {
		return fmt.Errorf("failed to update site status in database: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Printf("Site %s enabled successfully\n", domain)
	}

	return nil
}

// DisableSite disables a site by removing the symlink and updating database
func (sm *SQLiteSiteManager) DisableSite(domain string) error {
	if sm.Config.Verbose {
		fmt.Printf("Disabling site: %s\n", domain)
	}

	// Get site from database
	site, err := sm.DB.GetSite(domain)
	if err != nil {
		return err
	}

	if !site.IsEnabled && !sm.Config.DryRun {
		return fmt.Errorf("site %s is not enabled", domain)
	}

	symlinkPath := filepath.Join(sm.Config.EnabledSites, domain)

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would remove symlink: %s\n", symlinkPath)
			fmt.Printf("Would update database to set site as disabled\n")
		}
		return nil
	}

	// Remove symlink
	if err := os.Remove(symlinkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove symlink: %v", err)
	}

	// Update database
	site.IsEnabled = false
	if err := sm.DB.UpdateSite(site); err != nil {
		return fmt.Errorf("failed to update site status in database: %v", err)
	}

	if sm.Config.Verbose {
		fmt.Printf("Site %s disabled successfully\n", domain)
	}

	return nil
}

// ListSites lists all sites from the database
func (sm *SQLiteSiteManager) ListSites() error {
	allSites, err := sm.DB.ListSites(nil)
	if err != nil {
		return fmt.Errorf("failed to list sites: %v", err)
	}

	enabledTrue := true
	enabledSites, err := sm.DB.ListSites(&enabledTrue)
	if err != nil {
		return fmt.Errorf("failed to list enabled sites: %v", err)
	}

	fmt.Println("Available sites:")
	for _, site := range allSites {
		status := "disabled"
		if site.IsEnabled {
			status = "enabled"
		}
		siteType := "PHP"
		if site.IsWordPress {
			siteType = "WordPress"
		}
		fmt.Printf("  %s (%s, %s)\n", site.Domain, siteType, status)
	}

	fmt.Println("\nEnabled sites:")
	for _, site := range enabledSites {
		siteType := "PHP"
		if site.IsWordPress {
			siteType = "WordPress"
		}
		fmt.Printf("  %s (%s)\n", site.Domain, siteType)
	}

	return nil
}

// AddBasicAuth adds basic authentication using SQLite database
func (sm *SQLiteSiteManager) AddBasicAuth(domain, path, username, password string) error {
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

	// Get site from database
	site, err := sm.DB.GetSite(domain)
	if err != nil {
		return err
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

	// Generate password hash
	hashedPassword, err := sm.generatePasswordHash(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Create basic auth record
	auth := &database.BasicAuth{
		SiteID:   site.ID,
		Path:     path,
		Username: username,
		Password: hashedPassword,
	}

	if err := sm.DB.CreateBasicAuth(auth); err != nil {
		return fmt.Errorf("failed to store basic auth in database: %v", err)
	}

	// Regenerate Caddy configuration
	configFile := filepath.Join(sm.Config.AvailableSites, domain)
	if err := sm.regenerateCaddyConfig(site.ID, configFile); err != nil {
		return fmt.Errorf("failed to regenerate Caddy config: %v", err)
	}

	// Reload Caddy
	if err := sm.reloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %v", err)
	}

	fmt.Printf("Basic auth added for %s at path %s\n", domain, path)
	return nil
}

// RemoveBasicAuth removes basic authentication using SQLite database
func (sm *SQLiteSiteManager) RemoveBasicAuth(domain, path string) error {
	if sm.Config.Verbose {
		fmt.Printf("Removing basic auth for %s from path %s\n", domain, path)
	}

	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Get site from database
	site, err := sm.DB.GetSite(domain)
	if err != nil {
		return err
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would remove basic auth from path: %s\n", path)
		}
		return nil
	}

	// Remove basic auth records for this path
	if err := sm.DB.DeleteBasicAuthsForPath(site.ID, path); err != nil {
		return fmt.Errorf("failed to remove basic auth from database: %v", err)
	}

	// Regenerate Caddy configuration
	configFile := filepath.Join(sm.Config.AvailableSites, domain)
	if err := sm.regenerateCaddyConfig(site.ID, configFile); err != nil {
		return fmt.Errorf("failed to regenerate Caddy config: %v", err)
	}

	// Reload Caddy
	if err := sm.reloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %v", err)
	}

	fmt.Printf("Basic auth removed for %s from path %s\n", domain, path)
	return nil
}

// ModifyMaxUpload changes the maximum upload size using SQLite database
func (sm *SQLiteSiteManager) ModifyMaxUpload(domain, newSize string) error {
	if sm.Config.Verbose {
		fmt.Printf("Modifying max upload size for %s to %s\n", domain, newSize)
	}

	// Validate size format
	if err := sm.validateSizeFormat(newSize); err != nil {
		return fmt.Errorf("invalid size format: %v", err)
	}

	// Get site from database
	site, err := sm.DB.GetSite(domain)
	if err != nil {
		return err
	}

	if sm.Config.DryRun {
		if sm.Config.Verbose {
			fmt.Printf("Would modify max upload size:\n")
			fmt.Printf("  Domain: %s\n", domain)
			fmt.Printf("  New size: %s\n", newSize)
			fmt.Printf("  PHP-FPM pool: %s\n", site.PoolName)
		}
		return nil
	}

	// Update site in database
	site.MaxUpload = newSize
	if err := sm.DB.UpdateSite(site); err != nil {
		return fmt.Errorf("failed to update site in database: %v", err)
	}

	// Update PHP-FPM pool configuration
	if err := sm.updatePHPPoolUploadSize(site, newSize); err != nil {
		return fmt.Errorf("failed to update PHP pool: %v", err)
	}

	// Regenerate Caddy configuration
	configFile := filepath.Join(sm.Config.AvailableSites, domain)
	if err := sm.regenerateCaddyConfig(site.ID, configFile); err != nil {
		return fmt.Errorf("failed to regenerate Caddy config: %v", err)
	}

	// Restart PHP-FPM
	if err := sm.restartPHPFPM(site.PHPVersion); err != nil {
		return fmt.Errorf("failed to restart PHP-FPM: %v", err)
	}

	// Reload Caddy
	if err := sm.reloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %v", err)
	}

	fmt.Printf("Max upload size updated to %s for %s\n", newSize, domain)
	return nil
}

// Helper methods (implementing the rest of the functionality from the original manager)
// These will be similar to the original but simplified since we don't need config file parsing
