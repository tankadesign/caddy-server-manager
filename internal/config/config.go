package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// CaddyConfig represents the configuration for Caddy management
type CaddyConfig struct {
	ConfigDir      string
	AvailableSites string
	EnabledSites   string
	CaddyFile      string
	WebRoot        string
	PHPVersion     string
	DryRun         bool
	Verbose        bool
}

// NewCaddyConfig creates a new CaddyConfig with default values
func NewCaddyConfig(configDir string) *CaddyConfig {
	return &CaddyConfig{
		ConfigDir:      configDir,
		AvailableSites: filepath.Join(configDir, "available-sites"),
		EnabledSites:   filepath.Join(configDir, "enabled-sites"),
		CaddyFile:      filepath.Join(configDir, "Caddyfile"),
		WebRoot:        "/var/www",
		PHPVersion:     "8.2",
		DryRun:         false,
		Verbose:        false,
	}
}

// Validate checks if the configuration is valid
func (c *CaddyConfig) Validate() error {
	// Check if Caddy config directory exists
	if _, err := os.Stat(c.ConfigDir); os.IsNotExist(err) {
		return fmt.Errorf("caddy config directory does not exist: %s", c.ConfigDir)
	}

	// Create directories if they don't exist
	dirs := []string{c.AvailableSites, c.EnabledSites}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	return nil
}

// PrintConfig prints the current configuration if verbose mode is enabled
func (c *CaddyConfig) PrintConfig() {
	if c.Verbose {
		fmt.Printf("Caddy Config Directory: %s\n", c.ConfigDir)
		fmt.Printf("Available Sites: %s\n", c.AvailableSites)
		fmt.Printf("Enabled Sites: %s\n", c.EnabledSites)
		fmt.Printf("Caddyfile: %s\n", c.CaddyFile)
		fmt.Printf("Web Root: %s\n", c.WebRoot)
		fmt.Printf("PHP Version: %s\n", c.PHPVersion)
		fmt.Printf("Dry Run: %t\n", c.DryRun)
		fmt.Printf("Verbose: %t\n", c.Verbose)
	}
}
