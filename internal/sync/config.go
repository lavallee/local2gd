package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
)

// LoadConfig reads the config file and returns all pairing configurations.
func LoadConfig() ([]PairingConfig, error) {
	v := viper.New()
	v.SetConfigType("toml")

	var configPath string
	found := false
	for _, dir := range ConfigDirs() {
		configPath = filepath.Join(dir, "local2gd", "config.toml")
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err == nil {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("no config file found.\n\nCreate ~/.config/local2gd/config.toml:\n\n  mkdir -p ~/.config/local2gd\n  cat > ~/.config/local2gd/config.toml << 'EOF'\n  [auth]\n  client_id = \"your-id\"\n  client_secret = \"your-secret\"\n\n  [pairings.notes]\n  local = \"~/Documents/notes\"\n  remote = \"Notes\"\n  EOF")
	}

	pairings := v.GetStringMap("pairings")
	if len(pairings) == 0 {
		return nil, fmt.Errorf("no pairings defined in config file %s", configPath)
	}

	var configs []PairingConfig
	for name := range pairings {
		local := v.GetString(fmt.Sprintf("pairings.%s.local", name))
		remote := v.GetString(fmt.Sprintf("pairings.%s.remote", name))

		if local == "" || remote == "" {
			return nil, fmt.Errorf("pairing '%s' must have both 'local' and 'remote' fields", name)
		}

		configs = append(configs, PairingConfig{
			Name:       name,
			LocalDir:   local,
			RemotePath: remote,
		})
	}

	return configs, nil
}

// FindPairing returns the named pairing from the config, or an error if not found.
func FindPairing(configs []PairingConfig, name string) (*PairingConfig, error) {
	for i := range configs {
		if configs[i].Name == name {
			return &configs[i], nil
		}
	}

	var names []string
	for _, c := range configs {
		names = append(names, c.Name)
	}
	return nil, fmt.Errorf("pairing '%s' not found. Available: %v", name, names)
}

// ConfigDirs returns candidate config directories in priority order.
func ConfigDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{xdg.ConfigHome}
	dotConfig := filepath.Join(home, ".config")
	if dotConfig != xdg.ConfigHome {
		dirs = append(dirs, dotConfig)
	}
	return dirs
}

// ConfigPath returns the path where the config file should be.
func ConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "local2gd", "config.toml")
}
