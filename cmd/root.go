package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/geekxflood/program-director/internal/config"
)

var (
	cfgFile   string
	debug     bool
	dbDriver  string
	cfg       *config.Config
	logger    *slog.Logger
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "program-director",
	Short: "AI-powered TV channel programmer",
	Long: `Program Director generates themed playlists for your TV channels
using AI to select appropriate content from your media library.

It integrates with Radarr, Sonarr, and Tunarr to create intelligent
programming schedules based on configurable themes.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for version command
		if cmd.Name() == "version" {
			return nil
		}
		return initConfig()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() error {
	return rootCmd.Execute()
}

// SetVersionInfo sets the version information from build flags
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	buildDate = d
}

func init() {
	// Persistent flags available to all commands
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().StringVar(&dbDriver, "db-driver", "", "database driver override (postgres/sqlite)")

	// Bind flags to viper
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("database.driver", rootCmd.PersistentFlags().Lookup("db-driver"))

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(serveCmd)
}

func initConfig() error {
	// Initialize logger
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Load configuration
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.Debug("configuration loaded", "config_file", cfgFile)
	return nil
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("program-director %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", buildDate)
	},
}
