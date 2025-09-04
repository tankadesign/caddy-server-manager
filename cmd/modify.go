package cmd

import (
	"fmt"

	"github.com/falcon/caddy-site-manager/internal/config"
	"github.com/falcon/caddy-site-manager/internal/site"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Create site manager
		sm, err := site.NewCaddySiteManager(cfg)
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

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Create site manager
		sm, err := site.NewCaddySiteManager(cfg)
		if err != nil {
			return err
		}

		return sm.RemoveBasicAuth(domain, path)
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

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Create site manager
		sm, err := site.NewCaddySiteManager(cfg)
		if err != nil {
			return err
		}

		return sm.ModifyMaxUpload(domain, newSize)
	},
}

func init() {
	rootCmd.AddCommand(authAddCmd)
	rootCmd.AddCommand(authRemoveCmd)
	rootCmd.AddCommand(maxUploadCmd)

	// Add flags for auth-add command
	authAddCmd.Flags().StringP("username", "u", "", "Username for basic auth")
	authAddCmd.Flags().StringP("password", "p", "", "Password for basic auth")
}
