package config

import (
	"os"

	"github.com/jesseduffield/yaml"
)

// UserConfig holds user-editable settings
type UserConfig struct {
	DefaultLocalPath  string `yaml:"default_local_path"`
	DefaultRemotePath string `yaml:"default_remote_path"`
	SyncDebounceMs    int    `yaml:"sync_debounce_ms"`
	Theme             string `yaml:"theme"`
}

// LoadUserConfig loads the user config from file
func LoadUserConfig(configFile string) (*UserConfig, error) {
	config := &UserConfig{
		DefaultLocalPath:  os.ExpandEnv("$HOME"),
		DefaultRemotePath: "/home",
		SyncDebounceMs:    500,
		Theme:             "default",
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		// File doesn't exist yet, return defaults
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// Save writes the user config to file
func (c *UserConfig) Save(configFile string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}
