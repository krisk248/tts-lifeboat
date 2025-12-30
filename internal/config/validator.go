package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult holds the result of configuration validation.
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []string
}

// Validate checks the configuration for errors and warnings.
func (c *Config) Validate() *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []string{},
	}

	// Required fields
	if strings.TrimSpace(c.Name) == "" {
		result.addError("name", "instance name is required")
	}

	if strings.TrimSpace(c.WebappsPath) == "" {
		result.addError("webapps_path", "webapps path is required")
	} else {
		// Check if webapps path exists
		if _, err := os.Stat(c.WebappsPath); os.IsNotExist(err) {
			result.addError("webapps_path", fmt.Sprintf("path does not exist: %s", c.WebappsPath))
		}
	}

	// Validate webapps list
	if len(c.Webapps) == 0 {
		result.addWarning("No webapps specified; all apps in webapps_path will be backed up")
	} else {
		// Check if specified webapps exist
		for _, webapp := range c.Webapps {
			webappPath := filepath.Join(c.WebappsPath, webapp)
			if _, err := os.Stat(webappPath); os.IsNotExist(err) {
				result.addWarning(fmt.Sprintf("webapp '%s' does not exist at %s", webapp, webappPath))
			}
		}
	}

	// Validate custom folders
	for i, folder := range c.CustomFolders {
		if strings.TrimSpace(folder.Title) == "" {
			result.addError(fmt.Sprintf("custom_folders[%d].title", i), "folder title is required")
		}
		if strings.TrimSpace(folder.Path) == "" {
			result.addError(fmt.Sprintf("custom_folders[%d].path", i), "folder path is required")
		} else {
			// Check if path exists
			if _, err := os.Stat(folder.Path); os.IsNotExist(err) {
				if folder.Required {
					result.addError(fmt.Sprintf("custom_folders[%d].path", i),
						fmt.Sprintf("required folder does not exist: %s", folder.Path))
				} else {
					result.addWarning(fmt.Sprintf("optional folder '%s' does not exist: %s", folder.Title, folder.Path))
				}
			}
		}
	}

	// Validate retention settings
	if c.Retention.Enabled {
		if c.Retention.Days < 1 {
			result.addError("retention.days", "retention days must be at least 1")
		}
		if c.Retention.MinKeep < 0 {
			result.addError("retention.min_keep", "min_keep cannot be negative")
		}
	}

	// Validate compression settings
	if c.Compression.Enabled {
		if c.Compression.Level < 1 || c.Compression.Level > 9 {
			result.addError("compression.level", "compression level must be between 1 and 9")
		}
	}

	// Validate environment
	validEnvs := map[string]bool{
		"development": true, "dev": true,
		"staging": true, "stage": true,
		"production": true, "prod": true,
		"testing": true, "test": true,
	}
	if c.Environment != "" && !validEnvs[strings.ToLower(c.Environment)] {
		result.addWarning(fmt.Sprintf("unrecognized environment '%s'; consider using: dev, staging, production, testing", c.Environment))
	}

	return result
}

// addError adds an error and marks the result as invalid.
func (r *ValidationResult) addError(field, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{Field: field, Message: message})
}

// addWarning adds a warning without invalidating the result.
func (r *ValidationResult) addWarning(message string) {
	r.Warnings = append(r.Warnings, message)
}

// String returns a human-readable validation summary.
func (r *ValidationResult) String() string {
	var sb strings.Builder

	if r.Valid {
		sb.WriteString("Configuration is valid\n")
	} else {
		sb.WriteString("Configuration has errors:\n")
		for _, err := range r.Errors {
			sb.WriteString(fmt.Sprintf("  ✗ %s: %s\n", err.Field, err.Message))
		}
	}

	if len(r.Warnings) > 0 {
		sb.WriteString("Warnings:\n")
		for _, warn := range r.Warnings {
			sb.WriteString(fmt.Sprintf("  ⚠ %s\n", warn))
		}
	}

	return sb.String()
}

// MustValidate validates the config and returns an error if invalid.
func (c *Config) MustValidate() error {
	result := c.Validate()
	if !result.Valid {
		var errMsgs []string
		for _, e := range result.Errors {
			errMsgs = append(errMsgs, e.Error())
		}
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errMsgs, "; "))
	}
	return nil
}
