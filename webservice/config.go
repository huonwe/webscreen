package webservice

import (
	"log"
	"os"
	"scrmux/consts"

	"github.com/goccy/go-yaml"
)

type WebMasterConfig struct {
	EnableAndroidDiscover bool
	Host                  string `yaml:"host"`
	Port                  int    `yaml:"port"`

	JWTSecret  string   `yaml:"jwt_secret"`
	TrustedIPs []string `yaml:"trusted_ips"`
	PIN        string   `yaml:"pin"`
	MTU        uint16   `yaml:"mtu"`

	WEBRTC_UDP_RANGE_START uint16   `yaml:"webrtc_udp_range_start"`
	WEBRTC_UDP_RANGE_END   uint16   `yaml:"webrtc_udp_range_end"`
	ICE_SERVERS            []string `yaml:"ice_servers"`
}

func LoadConfig(path string) (WebMasterConfig, error) {
	defaultConfig := WebMasterConfig{
		EnableAndroidDiscover: true,
		Host:                  "0.0.0.0",
		Port:                  8081,
		JWTSecret:             "change_before_use",
		TrustedIPs:            []string{"127.0.0.1", "::1"},
		PIN:                   "000000",
		MTU:                   1400,

		WEBRTC_UDP_RANGE_START: 51200,
		WEBRTC_UDP_RANGE_END:   51210,
		ICE_SERVERS:            []string{"stun:stun.l.google.com:19302"},
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
	userConfig := defaultConfig
	if err := yaml.Unmarshal(data, &userConfig); err != nil {
		return defaultConfig, err
	}

	consts.WEBRTC_UDP_RANGE_START = userConfig.WEBRTC_UDP_RANGE_START
	consts.WEBRTC_UDP_RANGE_END = userConfig.WEBRTC_UDP_RANGE_END
	consts.WEBRTC_ICE_SERVERS = userConfig.ICE_SERVERS
	log.Println("Loaded user config: ", userConfig)
	return userConfig, nil
}

func SaveConfig(path string, config WebMasterConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
