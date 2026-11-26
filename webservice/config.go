package webservice

import (
	"log"
	"os"

	"github.com/goccy/go-yaml"
)

type WebMasterConfig struct {
	EnableAndroidDiscover bool
	Host                  string `yaml:"host"`
	Port                  int    `yaml:"port"`

	JWTSecret  string   `yaml:"jwt_secret"`
	TrustedIPs []string `yaml:"trusted_ips"`
	PIN        int      `yaml:"pin"`
	MTU        uint16   `yaml:"mtu"`
}

func LoadConfig(path string) (WebMasterConfig, error) {
	defaultConfig := WebMasterConfig{
		EnableAndroidDiscover: true,
		Host:                  "0.0.0.0",
		Port:                  8081,
		JWTSecret:             "change_before_use",
		TrustedIPs:            []string{"127.0.0.1", "::1"},
		PIN:                   0,
		MTU:                   1400,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Failed to read config file: %v\n", err)
		if os.IsNotExist(err) {
			// create a default config file
			if err := SaveConfig(path, defaultConfig); err != nil {
				log.Printf("Failed to save default config: %v\n", err)
				return defaultConfig, err
			}
			log.Printf("Created default config at %s\n", path)
		}
		return defaultConfig, nil
	}
	userConfig := WebMasterConfig{}
	if err := yaml.Unmarshal(data, &userConfig); err != nil {
		return defaultConfig, err
	}
	return userConfig, nil
}

func SaveConfig(path string, config WebMasterConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
