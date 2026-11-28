package gamestream

import (
	"fmt"
	"net"
	"scrmux/sdriver"
	"sync"
	"time"
)

var _ sdriver.SDriver = (*Driver)(nil)

type Driver struct {
	cfg       *streamConfig
	httpSess  *httpSession
	rtsp      *rtspResult
	ctrl      *controlStream
	videoConn *net.UDPConn
	audioConn *net.UDPConn

	videoChan   chan sdriver.AVBox
	audioChan   chan sdriver.AVBox
	msgChan     chan sdriver.Event
	controlChan chan sdriver.Event

	meta      sdriver.MediaMeta
	startTime time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	done      chan struct{}

	mu         sync.RWMutex
	lastVPS    []byte
	lastSPS    []byte
	lastPPS    []byte
	lastIDR    []byte
	lastIDRReq time.Time
	inputMu    sync.Mutex
	modifiers  byte
	padArrived [4]bool
}

// New initializes a GameStream / Sunshine streaming session (HTTP launch + RTSP handshake).
func New(config map[string]string) (*Driver, error) {
	cfg, err := parseConfig(config)
	if err != nil {
		return nil, err
	}
	store, err := loadOrCreateAuth(defaultAuthDir)
	if err != nil {
		return nil, fmt.Errorf("gamestream: init auth: %w", err)
	}
	hs, err := newHTTPSession(cfg, store)
	if err != nil {
		return nil, err
	}
	rtsp, err := performRTSPHandshake(hs, cfg)
	if err != nil {
		_ = hs.cancel()
		return nil, fmt.Errorf("gamestream: rtsp handshake failed: %w", err)
	}

	videoConn, err := net.ListenUDP("udp", nil)
	if err != nil {
		_ = hs.cancel()
		return nil, err
	}
	_ = videoConn.SetReadBuffer(4 * 1024 * 1024)
	audioConn, err := net.ListenUDP("udp", nil)
	if err != nil {
		_ = videoConn.Close()
		_ = hs.cancel()
		return nil, err
	}
	_ = audioConn.SetReadBuffer(512 * 1024)

	controlV2 := (rtsp.encryptionEnabled & ssEncControlV2) != 0
	ctrlEnc := hs.appVersionAtLeast(7, 1, 431) || controlV2
	ctrl, err := startControlStream(cfg.ip, rtsp.controlPort, rtsp.controlConnectData, hs.rikey, ctrlEnc, controlV2)
	if err != nil {
		_ = videoConn.Close()
		_ = audioConn.Close()
		_ = hs.cancel()
		return nil, fmt.Errorf("gamestream: control stream failed: %w", err)
	}

	return &Driver{
		cfg:         cfg,
		httpSess:    hs,
		rtsp:        rtsp,
		ctrl:        ctrl,
		videoConn:   videoConn,
		audioConn:   audioConn,
		videoChan:   make(chan sdriver.AVBox, 64),
		audioChan:   make(chan sdriver.AVBox, 64),
		msgChan:     make(chan sdriver.Event, 16),
		controlChan: make(chan sdriver.Event, 16),
		meta: sdriver.MediaMeta{
			VideoCodec: cfg.codec,
			AudioCodec: "opus",
			Width:      uint32(cfg.width),
			Height:     uint32(cfg.height),
			FPS:        uint32(cfg.fps),
		},
		done: make(chan struct{}),
	}, nil
}

func (d *Driver) GetReceivers() (<-chan sdriver.AVBox, <-chan sdriver.AVBox, chan sdriver.Event, chan sdriver.Event) {
	return d.videoChan, d.audioChan, d.msgChan, d.controlChan
}

func (d *Driver) Start() error {
	d.startOnce.Do(func() {
		d.startTime = time.Now()
		videoAddr, _ := net.ResolveUDPAddr("udp", net.JoinHostPort(d.cfg.ip, itoa(d.rtsp.videoPort)))
		audioAddr, _ := net.ResolveUDPAddr("udp", net.JoinHostPort(d.cfg.ip, itoa(d.rtsp.audioPort)))
		go d.videoLoop(d.videoConn, videoAddr, d.rtsp, d.httpSess.rikey)
		go d.audioLoop(d.audioConn, audioAddr, d.rtsp, d.httpSess.rikey, d.httpSess.rikeyID)
	})
	return nil
}

func (d *Driver) Pause() {}

func (d *Driver) RequestIDR(force bool) {
	// Never replay a stale cached IDR into the live stream (which would cause the
	// video to jump backward in time). Always ask the host for a fresh IDR frame,
	// throttled to at most once every 500ms unless force is true.
	d.mu.Lock()
	now := time.Now()
	shouldSend := force || now.Sub(d.lastIDRReq) >= 500*time.Millisecond
	if shouldSend {
		d.lastIDRReq = now
	}
	d.mu.Unlock()

	if shouldSend && d.ctrl != nil {
		_ = d.ctrl.requestIDR()
	}
}

func (d *Driver) Capabilities() sdriver.DriverCaps {
	return sdriver.DriverCaps{
		CanVideo:   true,
		CanAudio:   true,
		CanControl: true,
		IsWindows:  true,
	}
}

func (d *Driver) MediaMeta() sdriver.MediaMeta {
	return d.meta
}

func (d *Driver) Stop() {
	d.stopOnce.Do(func() {
		close(d.done)
		if d.ctrl != nil {
			d.ctrl.close()
		}
		if d.videoConn != nil {
			_ = d.videoConn.Close()
		}
		if d.audioConn != nil {
			_ = d.audioConn.Close()
		}
		if d.httpSess != nil {
			_ = d.httpSess.cancel()
		}
		close(d.controlChan)
	})
}
