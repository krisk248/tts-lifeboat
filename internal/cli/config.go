package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kannan/tts-lifeboat/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	Long:  "Commands for managing lifeboat configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new configuration file",
	Long: `Create a new lifeboat.yaml configuration file with defaults.

Examples:
  lifeboat config init
  lifeboat config init --name my-webapp
  lifeboat config init --webapps-path /path/to/webapps`,
	RunE: runConfigInit,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long: `Validate the configuration file and check paths.

Examples:
  lifeboat config validate
  lifeboat config validate -c /path/to/lifeboat.yaml`,
	RunE: runConfigValidate,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  "Display the loaded configuration values",
	RunE:  runConfigShow,
}

var (
	configInitName        string
	configInitWebappsPath string
	configInitOutput      string
)

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configShowCmd)

	configInitCmd.Flags().StringVar(&configInitName, "name", "my-webapp", "instance name")
	configInitCmd.Flags().StringVar(&configInitWebappsPath, "webapps-path", "", "path to webapps directory")
	configInitCmd.Flags().StringVarP(&configInitOutput, "output", "o", "lifeboat.yaml", "output file path")
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	// Check if file already exists
	if _, err := os.Stat(configInitOutput); err == nil {
		return fmt.Errorf("config file already exists: %s\nUse --output to specify a different path", configInitOutput)
	}

	// Create default config
	cfg := config.DefaultConfig()
	cfg.Name = configInitName

	if configInitWebappsPath != "" {
		absPath, err := filepath.Abs(configInitWebappsPath)
		if err == nil {
			cfg.WebappsPath = absPath
		} else {
			cfg.WebappsPath = configInitWebappsPath
		}
	}

	// Generate YAML content with comments
	content := generateConfigYAML(cfg)

	if err := os.WriteFile(configInitOutput, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Configuration file created: %s\n\n", configInitOutput)
	fmt.Println("📝 Next steps:")
	fmt.Println("   1. Edit lifeboat.yaml and set your webapps_path")
	fmt.Println("   2. List specific webapps to backup (or leave empty for all)")
	fmt.Println("   3. Add any custom folders to backup")
	fmt.Println("   4. Run 'lifeboat config validate' to verify")
	fmt.Println("   5. Run 'lifeboat backup --dry-run' to preview")

	return nil
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	// cfg is already loaded in PersistentPreRunE
	result := cfg.Validate()

	fmt.Println("🔍 Configuration Validation")
	fmt.Printf("   File: %s\n\n", cfgFile)

	if result.Valid {
		fmt.Println("✅ Configuration is VALID")
	} else {
		fmt.Println("❌ Configuration is INVALID")
	}

	fmt.Println()
	fmt.Print(result.String())

	if !result.Valid {
		return fmt.Errorf("configuration validation failed")
	}

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	fmt.Println("🚢 TTS Lifeboat - Configuration")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Instance:       %s\n", cfg.Name)
	fmt.Printf("  Environment:    %s\n", cfg.Environment)
	fmt.Printf("  Webapps Path:   %s\n", cfg.WebappsPath)
	fmt.Printf("  Backup Path:    %s\n", cfg.BackupPath)
	fmt.Println("═══════════════════════════════════════════════════════════")

	if len(cfg.Webapps) > 0 {
		fmt.Println()
		fmt.Println("📂 Webapps to backup:")
		for _, w := range cfg.Webapps {
			fmt.Printf("   • %s\n", w)
		}
	} else {
		fmt.Println()
		fmt.Println("📂 Webapps: (all in webapps_path)")
	}

	if len(cfg.CustomFolders) > 0 {
		fmt.Println()
		fmt.Println("📁 Custom folders:")
		for _, f := range cfg.CustomFolders {
			req := ""
			if f.Required {
				req = " [required]"
			}
			fmt.Printf("   • %s: %s%s\n", f.Title, f.Path, req)
		}
	}

	fmt.Println()
	fmt.Println("⚙️  Retention:")
	fmt.Printf("   • Enabled: %v\n", cfg.Retention.Enabled)
	fmt.Printf("   • Days:    %d\n", cfg.Retention.Days)
	fmt.Printf("   • MinKeep: %d\n", cfg.Retention.MinKeep)

	fmt.Println()
	fmt.Println("🗜️  Compression:")
	fmt.Printf("   • Enabled: %v\n", cfg.Compression.Enabled)
	fmt.Printf("   • Level:   %d\n", cfg.Compression.Level)

	return nil
}

func generateConfigYAML(cfg *config.Config) string {
	return fmt.Sprintf(`# TTS Lifeboat Configuration
# Created by Kannan

# Instance identification
name: "%s"
environment: "production"

# Path to Tomcat webapps directory (REQUIRED - use full path)
webapps_path: "%s"

# Backup destination (. = same folder as lifeboat.exe)
backup_path: "."

# List specific webapps to backup (leave empty for all)
webapps:
  # - "MyApp.war"
  # - "MyApp"
  # - "OtherApp"

# Additional folders to backup alongside webapps
custom_folders:
  # - title: "Tomcat Config"
  #   path: "C:\\path\\to\\Tomcat\\conf"
  #   required: true
  # - title: "Shared Configs"
  #   path: "C:\\path\\to\\shared"
  #   required: false

# Retention policy
retention:
  enabled: true
  days: 30
  min_keep: 5

# Compression settings
compression:
  enabled: true
  level: 6
  skip_extensions:
    - ".war"
    - ".jar"
    - ".zip"
    - ".gz"

# Logging settings
logging:
  path: "./logs/lifeboat.log"
  level: "info"
  max_size: "10MB"
  max_files: 5
`, cfg.Name, cfg.WebappsPath)
}
