package config

import (
	"os"
	"path/filepath"

	"github.com/OpenPeeDeeP/xdg"
)

// AppConfig holds build metadata and application paths
type AppConfig struct {
	Version   string
	Commit    string
	BuildDate string

	ConfigDir  string
	CacheDir   string
	LogFile    string
	HostsFile  string
}

// NewAppConfig creates and initializes the app configuration
func NewAppConfig() (*AppConfig, error) {
	xdgDirs := xdg.New("lazyscpsync", "0.1.0")

	configDir := filepath.Join(xdgDirs.ConfigHome(), "lazyscpsync")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	cacheDir := filepath.Join(xdgDirs.CacheHome(), "lazyscpsync")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	return &AppConfig{
		Version:   "0.1.0",
		Commit:    "dev",
		BuildDate: "dev",
		ConfigDir: configDir,
		CacheDir:  cacheDir,
		LogFile:   filepath.Join(cacheDir, "lazyscpsync.log"),
		HostsFile: filepath.Join(configDir, "hosts.yml"),
	}, nil
}
