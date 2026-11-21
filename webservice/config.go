package webservice

import (
	"os"

	"github.com/goccy/go-yaml"
)

type WebMasterConfig struct {
	EnableAndroidDiscover bool
	PIN                   string   `yaml:"pin"`
	JWTSecret             string   `yaml:"jwt_secret"`
	TrustedIPs            []string `yaml:"trusted_ips"`
	MTU                   uint16   `yaml:"mtu"`
}

func LoadConfig(path string) (WebMasterConfig, error) {
	config := WebMasterConfig{EnableAndroidDiscover: true}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, err
	}
	return config, nil
}
