package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tankadesign/caddy-site-manager/internal/config"
	"github.com/tankadesign/caddy-site-manager/internal/site"
)

// Auth commands
var authAddCmd = &cobra.Command{
	Use:   "auth-add [domain] [path]",
	Short: "Add basic authentication to a site path",
	Long: `Add basic authentication to a specific path in a site.

Examples:
  caddy-site-manager auth-add example.com "/admin" -u admin -p secret123
  caddy-site-manager auth-add blog.com "/wp-admin" -u wordpress -p mypassword`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]
		path := args[1]

		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")

		if username == "" || password == "" {
			return fmt.Errorf("username (-u) and password (-p) are required")
		}

		// Create config
		cfg := config.NewCaddyConfig(viper.GetString("caddy-config"))
		cfg.DryRun = viper.GetBool("dry-run")
		cfg.Verbose = viper.GetBool("verbose")
		
		// Set database path if provided
		if dbPath := viper.GetString("database"); dbPath != "" {
			cfg.DatabasePath = dbPath
		}

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Create SQLite site manager
		sm, err := site.NewManager(cfg)
		if err != nil {
			return err
		}

		return sm.AddBasicAuth(domain, path, username, password)
	},
}

var authRemoveCmd = &cobra.Command{
	Use:   "auth-remove [domain] [path]",
	Short: "Remove basic authentication from a site path",
	Long: `Remove basic authentication from a specific path in a site.

Examples:
  caddy-site-manager auth-remove example.com "/admin"
  caddy-site-manager auth-remove blog.com "/wp-admin"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]
		path := args[1]

		// Create config
		cfg := config.NewCaddyConfig(viper.GetString("caddy-config"))
		cfg.DryRun = viper.GetBool("dry-run")
		cfg.Verbose = viper.GetBool("verbose")
		
		// Set database path if provided
		if dbPath := viper.GetString("database"); dbPath != "" {
			cfg.DatabasePath = dbPath
		}

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Create SQLite site manager
		sm, err := site.NewManager(cfg)
		if err != nil {
			return err
		}

		return sm.RemoveBasicAuth(domain, path)
	},
}

var authListCmd = &cobra.Command{
	Use:   "auth-list [domain]",
	Short: "List basic authentication configurations for a site",
	Long: `List all basic authentication paths and users configured for a site.

Examples:
  caddy-site-manager auth-list example.com
  caddy-site-manager auth-list blog.com`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]

		// Create config
		cfg := config.NewCaddyConfig(viper.GetString("caddy-config"))
		cfg.DryRun = viper.GetBool("dry-run")
		cfg.Verbose = viper.GetBool("verbose")
		
		// Set database path if provided
		if dbPath := viper.GetString("database"); dbPath != "" {
			cfg.DatabasePath = dbPath
		}

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Create SQLite site manager
		sm, err := site.NewManager(cfg)
		if err != nil {
			return err
		}

		return sm.ListBasicAuth(domain)
	},
}

var maxUploadCmd = &cobra.Command{
	Use:   "max-upload [domain] [size]",
	Short: "Change maximum upload size for a site",
	Long: `Change the maximum upload size for both PHP-FPM and Caddy configurations.

The size can be specified in various formats:
- 100M (megabytes)
- 2G or 2GB (gigabytes)
- 512MB (megabytes)

Examples:
  caddy-site-manager max-upload example.com 2GB
  caddy-site-manager max-upload blog.com 512M`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]
		newSize := args[1]

		// Create config
		cfg := config.NewCaddyConfig(viper.GetString("caddy-config"))
		cfg.DryRun = viper.GetBool("dry-run")
		cfg.Verbose = viper.GetBool("verbose")
		
		// Set database path if provided
		if dbPath := viper.GetString("database"); dbPath != "" {
			cfg.DatabasePath = dbPath
		}

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Create SQLite site manager
		sm, err := site.NewManager(cfg)
		if err != nil {
			return err
		}

		return sm.ModifyMaxUpload(domain, newSize)
	},
}

func init() {
	rootCmd.AddCommand(authAddCmd)
	rootCmd.AddCommand(authRemoveCmd)
	rootCmd.AddCommand(authListCmd)
	rootCmd.AddCommand(maxUploadCmd)

	// Add flags for auth-add command
	authAddCmd.Flags().StringP("username", "u", "", "Username for basic auth")
	authAddCmd.Flags().StringP("password", "p", "", "Password for basic auth")
}
