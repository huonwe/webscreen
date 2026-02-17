package webservice

import (
	"fmt"
	"webscreen/sdriver"
	"webscreen/sdriver/dummy"
	"webscreen/sdriver/scrcpy"
	"webscreen/sdriver/sunshine"
	linuxXvfbDriver "webscreen/sdriver/xvfb"
	sagent "webscreen/streamAgent"
)

func InitDriver(config sagent.AgentConfig) (sdriver.SDriver, error) {
	var driver sdriver.SDriver
	var err error
	switch config.DriverConfig["driver"] {
	case "scrcpy":
		driver, err = scrcpy.New(config.DriverConfig)
	case "sunshine":
		sunshine.SSTest()
	case "xvfb":
		driver, err = linuxXvfbDriver.New(config.DriverConfig)
	case "dummy":
		driver, err = dummy.New(config.DriverConfig)
	default:
		return nil, fmt.Errorf("unsupported driver type: %s", config.DriverConfig["driver"])
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize driver: %w", err)
	}
	return driver, nil
}
