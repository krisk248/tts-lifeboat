// Package cli provides command-line interface commands for tts-lifeboat.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kannan/tts-lifeboat/internal/app"
	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/logger"
)

var (
	cfgFile string
	cfg     *config.Config
	verbose bool
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "lifeboat",
	Short: "TTS Lifeboat - Enterprise backup solution for Tomcat applications",
	Long: `TTS Lifeboat is an enterprise-grade backup solution for Tomcat web applications.

It provides:
  • Intelligent backup with smart compression (skips already compressed files)
  • Checkpoint backups that never auto-delete
  • Configurable retention policies
  • Support for custom folders alongside webapps
  • Both TUI and CLI interfaces

Created with ❤️ by Kannan`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for certain commands
		if cmd.Name() == "version" || cmd.Name() == "credits" || cmd.Name() == "init" || cmd.Name() == "help" {
			return nil
		}

		// Load configuration
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger
		logLevel := "info"
		if verbose {
			logLevel = "debug"
		}
		logCfg := logger.Config{
			Path:    cfg.Logging.Path,
			Level:   logLevel,
			Console: true,
		}
		if err := logger.Init(logCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize logger: %v\n", err)
		}

		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ./lifeboat.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(creditsCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(checkpointCmd)
}

// versionCmd shows version information.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Print detailed version information about TTS Lifeboat",
	Run: func(cmd *cobra.Command, args []string) {
		showVerbose, _ := cmd.Flags().GetBool("verbose")
		if showVerbose {
			fmt.Println(app.GetEasterEgg())
			fmt.Println(app.GetVersionInfo())
		} else {
			fmt.Printf("TTS Lifeboat v%s\n", app.Version)
		}
	},
}

func init() {
	versionCmd.Flags().Bool("verbose", false, "show verbose version info")
}

// creditsCmd shows credits (easter egg).
var creditsCmd = &cobra.Command{
	Use:    "credits",
	Short:  "Show credits",
	Hidden: true, // Easter egg!
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(app.GetCredits())
	},
}
