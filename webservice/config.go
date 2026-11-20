package webservice

import (
	"os"

	"github.com/goccy/go-yaml"
)

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
