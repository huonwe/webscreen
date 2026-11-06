package sagent

import (
	"fmt"
	"log"
	"time"
	"webscreen/sdriver"
	linuxDriver "webscreen/sdriver/linux"
	rockchipDriver "webscreen/sdriver/rockchip-hdmirx"
	"webscreen/sdriver/scrcpy"
	"webscreen/sdriver/sunshine"

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

	// WebRTC 相关
	videoTrack        *webrtc.TrackLocalStaticRTP
	audioTrack        *webrtc.TrackLocalStaticRTP
	startTime         time.Time
	useLocalTimestamp bool

	// WebSocket 回调
	OnVideoFrame func([]byte)
	OnAudioFrame func([]byte)
}

// ========================
// SAgent 负责初始化driver并接受来自sdriver的数据，并处理来自前端的控制命令。
// 提供一系列Hook
// ========================
func New(config AgentConfig, videoTrack *webrtc.TrackLocalStaticRTP, audioTrack *webrtc.TrackLocalStaticRTP) *Agent {
	sa := &Agent{
		config:            config,
		videoTrack:        videoTrack,
		audioTrack:        audioTrack,
		useLocalTimestamp: config.UseLocalTimestamp,
	}
	log.Printf("AVSync: %v, UseLocalTimestamp: %v", config.AVSync, config.UseLocalTimestamp)
	log.Printf("Driver config: %+v", config.DriverConfig)
	return sa
}

// driverInitAttempts 是 scrcpy 会话启动的最大尝试次数（首次 + 重试）。
const driverInitAttempts = 3

func (sa *Agent) InitDriver(finalCodec webrtc.RTPCodecParameters) error {
	sa.config.DriverConfig["webrtc_codec_level"] = fmt.Sprintf("%d||%s||%s", finalCodec.PayloadType, finalCodec.MimeType, finalCodec.SDPFmtpLine)
	switch sa.config.DeviceType {
	// case DEVICE_TYPE_DUMMY:
	// 	// 初始化 Dummy Driver
	// 	dummyDriver, err := dummy.New(sa.config.DriverConfig)
	// 	if err != nil {
	// 		log.Printf("Failed to initialize dummy driver: %v", err)
	// 		return err
	// 	}
	// 	sa.driver = dummyDriver
	case DEVICE_TYPE_ANDROID:
		// 初始化 Android Driver（失败有限重试）
		//
		// scrcpy 会话启动是一次性握手：设备端 scrcpy-server 起得慢、上一会话的
		// socket/端口尚未释放等，都会让读取设备元数据时直接拿到 EOF，用户看到的
		// 就是“服务器错误: failed to ensure agent: EOF”。这类失败多为瞬时且可恢复，
		// 而 scrcpy.New 的失败路径会释放 listener/隧道/连接，因此重试是安全的。
		sa.config.DriverConfig["deviceID"] = sa.config.DeviceID
		var lastErr error
		for attempt := 1; attempt <= driverInitAttempts; attempt++ {
			androidDriver, err := scrcpy.New(sa.config.DriverConfig)
			if err == nil {
				sa.driver = androidDriver
				if attempt > 1 {
					log.Printf("Android driver initialized on attempt %d/%d", attempt, driverInitAttempts)
				}
				break
			}
			lastErr = err
			log.Printf("Failed to initialize Android driver (attempt %d/%d): %v", attempt, driverInitAttempts, err)
			if attempt < driverInitAttempts {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			}
		}
		if sa.driver == nil {
			return lastErr
		}
	case DEVICE_TYPE_LINUX, "xvfb":
		// 初始化 Linux Driver
		driver, err := linuxDriver.New(sa.config.DriverConfig)
		if err != nil {
			log.Printf("Failed to initialize Linux driver: %v", err)
			return err
		}
		sa.driver = driver
	case DEVICE_TYPE_ROCKCHIP_HDMIRX:
		driver, err := rockchipDriver.New(sa.config.DriverConfig)
		if err != nil {
			log.Printf("Failed to initialize Rockchip HDMI RX driver: %v", err)
			return err
		}
		sa.driver = driver
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

// Alive reports whether the underlying driver session is still usable. It is
// used to drop a dead pipeline (crashed scrcpy server, broken socket) instead
// of reusing it for the next session.
func (sa *Agent) Alive() bool {
	if sa.driver == nil {
		return false
	}
	if a, ok := sa.driver.(interface{ Alive() bool }); ok {
		return a.Alive()
	}
	return true
}

func (sa *Agent) GetCodecInfo() (string, string) {
	m := sa.driver.MediaMeta()
	return m.VideoCodec, m.AudioCodec
}

// Notify asks the browser(s) to display a message (toast). It is used for
// situations that are handled automatically, e.g. falling back to another
// video codec, so the user is not left wondering why the stream differs from
// what was configured.
func (sa *Agent) Notify(msg string) {
	if sa.controlCh == nil {
		log.Printf("[agent] no control channel, cannot notify: %s", msg)
		return
	}
	select {
	case sa.controlCh <- sdriver.TextMsgEvent{Msg: msg}:
	default:
		log.Printf("[agent] dropped notification: %s", msg)
	}
}

func (sa *Agent) GetMediaMeta() sdriver.MediaMeta {
	return sa.driver.MediaMeta()
}

func (sa *Agent) Capabilities() sdriver.DriverCaps {
	return sa.driver.Capabilities()
}

func (sa *Agent) Start() {
	err := sa.driver.Start()
	if err != nil {
		log.Printf("Failed to start driver: %v", err)
		return
	}
	sa.startTime = time.Now() // 服务器基准时间线
	go sa.ServeVideoStream()
	go sa.ServeAudioStream()

	sa.driver.RequestIDR(true)
}

func (sa *Agent) PLIRequest() {
	sa.driver.RequestIDR(false)
}

func (sa *Agent) HandleEvent(raw []byte) error {
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
