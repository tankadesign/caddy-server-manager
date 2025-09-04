package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "caddy-site-manager",
		Short: "A CLI tool for managing Caddy PHP/WordPress sites",
		Long: `Caddy Site Manager is a CLI tool that helps you manage PHP and WordPress sites
with Caddy web server. It provides commands to create, enable, disable, and remove sites
with proper configuration management.`,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.caddy-site-manager.yaml)")
	rootCmd.PersistentFlags().StringP("caddy-config", "c", "/etc/caddy", "Path to Caddy configuration directory")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolP("dry-run", "n", false, "Show what would be done without executing")
	rootCmd.PersistentFlags().String("database", "", "Path to SQLite database file (default: caddy-config-dir/caddy-sites.db)")

	// Bind flags to viper
	viper.BindPFlag("caddy-config", rootCmd.PersistentFlags().Lookup("caddy-config"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("dry-run", rootCmd.PersistentFlags().Lookup("dry-run"))
	viper.BindPFlag("database", rootCmd.PersistentFlags().Lookup("database"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".caddy-site-manager")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}
