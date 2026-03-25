package sync

import (
	"fmt"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
)

// LoadConfig reads the config file and returns all pairing configurations.
func LoadConfig() ([]PairingConfig, error) {
	configPath := filepath.Join(xdg.ConfigHome, "local2gd", "config.toml")

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("toml")

	if err := v.ReadInConfig(); err != nil {
		dir := filepath.Dir(configPath)
		return nil, fmt.Errorf("failed to read config at %s: %w\n\nCreate a config file:\n\n  mkdir -p %s\n  cat > %s << 'EOF'\n  [pairings.notes]\n  local = \"~/Documents/notes\"\n  remote = \"Notes\"\n  EOF",
			configPath, err, dir, configPath)
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

// ConfigPath returns the path where the config file should be.
func ConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "local2gd", "config.toml")
}
