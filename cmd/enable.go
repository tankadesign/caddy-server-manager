package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tankadesign/caddy-site-manager/internal/config"
	"github.com/tankadesign/caddy-site-manager/internal/site"
)

var enableCmd = &cobra.Command{
	Use:   "enable [domain]",
	Short: "Enable a site by creating a symlink",
	Long:  `Enable a site by creating a symlink from enabled-sites to available-sites.`,
	Args:  cobra.ExactArgs(1),
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

		// Enable site
		return sm.EnableSite(domain)
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable [domain]",
	Short: "Disable a site by removing the symlink",
	Long:  `Disable a site by removing the symlink from enabled-sites.`,
	Args:  cobra.ExactArgs(1),
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

		// Disable site
		return sm.DisableSite(domain)
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete [domain]",
	Short: "Delete a site configuration and optionally all related files",
	Long: `Delete a site configuration and optionally all related files.

Without --hard: Only removes symlink from enabled-sites
With --hard: Removes symlink, deletes config file, removes database (if WordPress), removes directory, removes PHP-FPM pool`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]

		// Get flags
		hard, _ := cmd.Flags().GetBool("hard")
		force, _ := cmd.Flags().GetBool("force")

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

		// Delete site options
		opts := &site.SiteDeleteOptions{
			Domain: domain,
			Hard:   hard,
			Force:  force,
		}

		// Delete site
		return sm.DeleteSite(opts)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available and enabled sites",
	Long:  `List all available and enabled sites.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// List sites
		return sm.ListSites()
	},
}

func init() {
	rootCmd.AddCommand(enableCmd)
	rootCmd.AddCommand(disableCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listCmd)

	// Add flags for delete command
	deleteCmd.Flags().Bool("hard", false, "Perform complete deletion (see description)")
	deleteCmd.Flags().Bool("force", false, "Force deletion without confirmation")
}
