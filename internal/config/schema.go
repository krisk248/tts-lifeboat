package config

type Config struct {
	Name          string   `toml:"name"`
	WebappsPath   string   `toml:"webapps_path"`
	BackupPath    string   `toml:"backup_path"`
	Compression   bool     `toml:"compression"`
	RetentionDays int      `toml:"retention_days"`
	ExtraFolders  []string `toml:"extra_folders"`
}

func Default() *Config {
	return &Config{
		Name:          "my-webapp",
		WebappsPath:   "",
		BackupPath:    ".",
		Compression:   defaultCompression(),
		RetentionDays: 30,
		ExtraFolders:  []string{},
	}
}
