package cmd

import (
	"github.com/falcon/caddy-site-manager/internal/config"
	"github.com/falcon/caddy-site-manager/internal/site"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createCmd = &cobra.Command{
	Use:   "create [domain]",
	Short: "Create a new PHP or WordPress site",
	Long: `Create a new PHP or WordPress site with Caddy configuration and custom PHP-FPM pool.

Examples:
  caddy-site-manager create mysite.com --wordpress --db=mysite_db --pwd=secure_password
  caddy-site-manager create mysite.com --wordpress
  caddy-site-manager create phpsite.com --max-upload=512M
  caddy-site-manager create basicsite.com`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]

		// Get flags
		wordpress, _ := cmd.Flags().GetBool("wordpress")
		dbName, _ := cmd.Flags().GetString("db")
		dbPassword, _ := cmd.Flags().GetString("pwd")
		maxUpload, _ := cmd.Flags().GetString("max-upload")
		phpVersion, _ := cmd.Flags().GetString("php")

		// Create config
		cfg := config.NewCaddyConfig(viper.GetString("caddy-config"))
		cfg.DryRun = viper.GetBool("dry-run")
		cfg.Verbose = viper.GetBool("verbose")
		cfg.PHPVersion = phpVersion

		if err := cfg.Validate(); err != nil {
			return err
		}

		if cfg.Verbose {
			cfg.PrintConfig()
		}

		// Create site manager
		sm, err := site.NewCaddySiteManager(cfg)
		if err != nil {
			return err
		}

		// Create site options
		opts := &site.SiteCreateOptions{
			Domain:     domain,
			WordPress:  wordpress,
			DBName:     dbName,
			DBPassword: dbPassword,
			MaxUpload:  maxUpload,
			PHPVersion: phpVersion,
		}

		// Create site
		return sm.CreateSite(opts)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().Bool("wordpress", false, "Setup WordPress (requires database)")
	createCmd.Flags().String("db", "", "Database name (auto-generated if not provided with --wordpress)")
	createCmd.Flags().String("pwd", "", "Database password (auto-generated if not provided with --wordpress)")
	createCmd.Flags().String("max-upload", "256M", "Maximum upload size")
	createCmd.Flags().String("php", "8.3", "PHP version to use")
}
