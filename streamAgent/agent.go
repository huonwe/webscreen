package sagent

import (
	"fmt"
	"log"
	"time"
	"webscreen/sdriver"
	"webscreen/sdriver/dummy"
	"webscreen/sdriver/scrcpy"
	"webscreen/sdriver/sunshine"
	linuxXvfbDriver "webscreen/sdriver/xvfb"

	"github.com/pion/webrtc/v4"
)

type Agent struct {
	driver     sdriver.SDriver
	driverCaps sdriver.DriverCaps
	config     AgentConfig
	// chan
	videoCh   <-chan sdriver.AVBox
	audioCh   <-chan sdriver.AVBox
	controlCh chan sdriver.Event

	// 用于音视频推流的 PTS 记录
	lastVideoPTS time.Duration
	lastAudioPTS time.Duration
	baseTime     time.Time
}

// ========================
// SAgent 负责初始化driver并接受来自sdriver的数据，并处理来自前端的控制命令。
// 提供一系列Hook
// ========================
func New(config AgentConfig) *Agent {
	sa := &Agent{
		config: config,
	}
	log.Printf("Driver config: %+v", config.DriverConfig)
	return sa
}

func (sa *Agent) InitDriver(finalCodec webrtc.RTPCodecParameters) error {
	sa.config.DriverConfig["webrtc_codec_level"] = fmt.Sprintf("%d||%s||%s", finalCodec.PayloadType, finalCodec.MimeType, finalCodec.SDPFmtpLine)
	switch sa.config.DeviceType {
	case DEVICE_TYPE_DUMMY:
		// 初始化 Dummy Driver
		dummyDriver, err := dummy.New(sa.config.DriverConfig)
		if err != nil {
			log.Printf("Failed to initialize dummy driver: %v", err)
			return err
		}
		sa.driver = dummyDriver
	case DEVICE_TYPE_ANDROID:
		// 初始化 Android Driver
		sa.config.DriverConfig["deviceID"] = sa.config.DeviceID
		androidDriver, err := scrcpy.New(sa.config.DriverConfig)
		if err != nil {
			log.Printf("Failed to initialize Android driver: %v", err)
			return err
		}
		sa.driver = androidDriver
	case DEVICE_TYPE_XVFB:
		// 初始化 Linux Driver
		linuxDriver, err := linuxXvfbDriver.New(sa.config.DriverConfig)
		if err != nil {
			log.Printf("Failed to initialize Linux driver: %v", err)
			return err
		}
		sa.driver = linuxDriver
	case DEVICE_TYPE_SUNSHINE:
		sunshine.SSTest()
	default:
		log.Printf("Unsupported device type: %s", sa.config.DeviceType)
		return fmt.Errorf("unsupported device type: %s", sa.config.DeviceType)
	}
	sa.driverCaps = sa.driver.Capabilities()
	sa.videoCh, sa.audioCh, sa.controlCh = sa.driver.GetReceivers()
	return nil
}

func (sa *Agent) Close() {
	log.Printf("Closing agent for device %s", sa.config.DeviceID)
	if sa.driver != nil {
		sa.driver.Stop()
	}
}

func (sa *Agent) GetCodecInfo() (string, string) {
	m := sa.driver.MediaMeta()
	return m.VideoCodec, m.AudioCodec
}

func (sa *Agent) GetMediaMeta() sdriver.MediaMeta {
	return sa.driver.MediaMeta()
}

func (sa *Agent) Capabilities() sdriver.DriverCaps {
	return sa.driver.Capabilities()
}

func (sa *Agent) Start() {
	sa.driver.Start()
	sa.baseTime = time.Now()

	sa.driver.RequestIDR(true)
}

func (sa *Agent) PLIRequest() {
	sa.driver.RequestIDR(false)
}

func (sa *Agent) SendEvent(raw []byte) error {
	if !sa.driverCaps.CanControl {
		return fmt.Errorf("driver does not support control events")
	}
	event, err := sa.parseEvent(raw)
	if err != nil {
		log.Printf("[agent] Failed to parse control event: %v", err)
		return err
	}
	// log.Printf("Parsed control event: %+v", event)
	return sa.driver.SendEvent(event)
}
