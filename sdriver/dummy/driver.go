package dummy

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"webcpy/sdriver"
)

// DummyDriver implements sdriver.SDriver by reading from a local H.264 Annex B file.
type DummyDriver struct {
	filePath string
	fps      int

	mu       sync.RWMutex
	running  bool
	stopOnce sync.Once
	stopCh   chan struct{}

	videoCh   chan sdriver.AVBox
	audioCh   chan sdriver.AVBox
	controlCh chan sdriver.ControlEvent

	lastSPS []byte
	lastPPS []byte
}

// New creates a dummy driver to stream from a local H.264 file.
// fps defines the nominal frame rate used to compute timestamps for VCL NALs.
func New(c sdriver.StreamConfig) *DummyDriver {
	if c.Bitrate <= 0 {
		c.Bitrate = 30
	}
	return &DummyDriver{
		filePath:  c.OtherOpts["file"],
		fps:       c.Bitrate,
		stopCh:    make(chan struct{}),
		videoCh:   make(chan sdriver.AVBox, 64),
		audioCh:   make(chan sdriver.AVBox, 1),
		controlCh: make(chan sdriver.ControlEvent, 1),
	}
}

// StartStream starts reading the H.264 file and produces AVBox packets.
func (d *DummyDriver) StartStream(config sdriver.StreamConfig) (<-chan sdriver.AVBox, <-chan sdriver.AVBox, <-chan sdriver.ControlEvent, error) {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return d.videoCh, d.audioCh, d.controlCh, nil
	}
	// Optionally override from config.OtherOpts
	if config.OtherOpts != nil {
		if p, ok := config.OtherOpts["file"]; ok && p != "" {
			d.filePath = p
		}
		if v, ok := config.OtherOpts["fps"]; ok && v != "" {
			if parsed, err := parseFPS(v); err == nil && parsed > 0 {
				d.fps = parsed
			}
		}
	}
	if d.filePath == "" {
		d.mu.Unlock()
		return nil, nil, nil, errors.New("dummy: file path is empty")
	}
	d.running = true
	d.mu.Unlock()

	go d.loop()
	return d.videoCh, d.audioCh, d.controlCh, nil
}

// SendControl is a no-op for dummy driver.
func (d *DummyDriver) SendControl(event sdriver.ControlEvent) error { return nil }

// RequestIDR attempts to resend cached SPS/PPS instantly (if available).
func (d *DummyDriver) RequestIDR() error {
	d.mu.RLock()
	sps := append([]byte(nil), d.lastSPS...)
	pps := append([]byte(nil), d.lastPPS...)
	d.mu.RUnlock()

	if len(sps) > 0 {
		select {
		case d.videoCh <- sdriver.AVBox{Data: sps, PTS: 0, IsKeyFrame: false, IsConfig: true}:
		default:
		}
	}
	if len(pps) > 0 {
		select {
		case d.videoCh <- sdriver.AVBox{Data: pps, PTS: 0, IsKeyFrame: false, IsConfig: true}:
		default:
		}
	}
	return nil
}

// Capabilities reports what this driver supports.
func (d *DummyDriver) Capabilities() sdriver.DriverCaps {
	return sdriver.DriverCaps{CanClipboard: false, CanUHID: false, CanVideo: true, CanAudio: false, CanControl: false}
}

// CodecInfo returns the video/audio codec identifiers.
func (d *DummyDriver) CodecInfo() (string, string) { return "h264", "" }

// Stop stops the streaming loop and closes channels.
func (d *DummyDriver) Stop() error {
	d.stopOnce.Do(func() {
		close(d.stopCh)
		// Allow the loop to close the output channels
	})
	return nil
}

func (d *DummyDriver) loop() {
	defer func() {
		// Close channels on exit
		close(d.videoCh)
		close(d.audioCh)
		close(d.controlCh)
		d.mu.Lock()
		d.running = false
		d.mu.Unlock()
	}()

	frameDur := time.Second / time.Duration(d.fps)
	var pts time.Duration

	for {
		// Read entire file (simple for dummy)
		f, err := os.Open(d.filePath)
		if err != nil {
			return
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil || len(data) == 0 {
			return
		}

		nals := splitAnnexB(data)
		for _, nal := range nals {
			select {
			case <-d.stopCh:
				return
			default:
			}

			// Extract NAL type for H.264
			if len(nal) == 0 {
				continue
			}
			nalType := nal[0] & 0x1F
			isVCL := nalType == 1 || nalType == 5
			isIDR := nalType == 5
			isConfig := nalType == 7 || nalType == 8 // SPS / PPS

			switch nalType {
			case 6: // SEI
				// Ignore
				continue
			case 7:
				d.mu.Lock()
				d.lastSPS = append(d.lastSPS[:0], nal...)
				d.mu.Unlock()
			case 8:
				d.mu.Lock()
				d.lastPPS = append(d.lastPPS[:0], nal...)
				d.mu.Unlock()
			}

			// PTSing: advance on VCL NALs, keep config with current PTS
			ts := pts
			if isVCL {
				// Use current PTS, then advance for next frame
				pts += frameDur
			}

			box := sdriver.AVBox{
				Data:       nal,
				PTS:        ts,
				IsKeyFrame: isIDR,
				IsConfig:   isConfig,
			}

			// Non-blocking send to avoid deadlock on slow consumers
			select {
			case d.videoCh <- box:
			case <-d.stopCh:
				return
			}
		}
		// Loop playback
	}
}

// splitAnnexB splits an Annex B H.264 byte stream into NAL units (without start codes).
func splitAnnexB(b []byte) [][]byte {
	var out [][]byte
	i := 0
	for {
		start, scLen := findStartCode(b, i)
		if start < 0 {
			break
		}
		next, _ := findStartCode(b, start+scLen)
		if next < 0 {
			nal := trimTrailingZeros(b[start+scLen:])
			if len(nal) > 0 {
				out = append(out, nal)
			}
			break
		}
		nal := trimTrailingZeros(b[start+scLen : next])
		if len(nal) > 0 {
			out = append(out, nal)
		}
		i = next
	}
	return out
}

func findStartCode(b []byte, from int) (int, int) {
	n := len(b)
	for i := from; i+3 < n; i++ {
		// 4-byte start code 0x00000001
		if i+3 < n && b[i] == 0x00 && b[i+1] == 0x00 && b[i+2] == 0x00 && b[i+3] == 0x01 {
			return i, 4
		}
		// 3-byte start code 0x000001
		if b[i] == 0x00 && b[i+1] == 0x00 && b[i+2] == 0x01 {
			return i, 3
		}
	}
	return -1, 0
}

func trimTrailingZeros(b []byte) []byte {
	i := len(b)
	for i > 0 && b[i-1] == 0x00 {
		i--
	}
	return b[:i]
}

// func addStartCode(nal []byte) []byte {
// 	out := make([]byte, 4+len(nal))
// 	copy(out, []byte{0x00, 0x00, 0x00, 0x01})
// 	copy(out[4:], nal)
// 	return out
// }

func parseFPS(s string) (int, error) {
	// very small helper: accept simple integer
	var v int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errors.New("invalid fps")
		}
		v = v*10 + int(ch-'0')
	}
	if v <= 0 {
		return 0, errors.New("invalid fps")
	}
	return v, nil
}
