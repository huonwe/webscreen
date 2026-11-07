package rockchiphdmirx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"webscreen/sdriver"
)

const defaultFFmpeg = "/usr/lib/jellyfin-ffmpeg/ffmpeg"

type Driver struct {
	videoChan   chan sdriver.AVBox
	audioChan   chan sdriver.AVBox
	controlChan chan sdriver.Event

	cmd       *exec.Cmd
	stdout    io.ReadCloser
	stopOnce  sync.Once
	startOnce sync.Once
	done      chan struct{}

	meta sdriver.MediaMeta

	mu      sync.RWMutex
	lastVPS []byte
	lastSPS []byte
	lastPPS []byte
	lastIDR []byte
}

// New creates an HDMI RX capture pipeline. All capture and encoding options are
// intentionally resolved here so a driver cannot be created with a partial
// configuration.
func New(config map[string]string) (*Driver, error) {
	device, err := findDevice(config["device"])
	if err != nil {
		return nil, err
	}
	configWidth, configHeight, err := parseResolution(config["resolution"])
	if err != nil {
		return nil, err
	}
	width, height := activeResolution(device, configWidth, configHeight)
	if width != configWidth || height != configHeight {
		log.Printf("[rockchip-hdmirx] using active HDMI resolution %dx%d instead of configured %dx%d",
			width, height, configWidth, configHeight)
	}
	fps, err := parsePositiveInt(config["frame_rate"], 60, "frame_rate")
	if err != nil || fps > 120 {
		return nil, fmt.Errorf("frame_rate must be between 1 and 120")
	}
	codec := strings.ToLower(config["video_codec"])
	if codec == "" {
		codec = "h264"
	}
	if codec != "h264" && codec != "h265" {
		return nil, fmt.Errorf("video_codec must be h264 or h265")
	}
	bitRate := config["video_bit_rate"]
	if bitRate == "" {
		bitRate = "8M"
	}
	ffmpeg := config["ffmpeg"]
	if ffmpeg == "" {
		ffmpeg = defaultFFmpeg
	}
	if _, err := os.Stat(ffmpeg); err != nil {
		return nil, fmt.Errorf("ffmpeg executable %q is unavailable: %w", ffmpeg, err)
	}

	encoder := "h264_rkmpp"
	outputFormat := "h264"
	if codec == "h265" {
		encoder = "hevc_rkmpp"
		outputFormat = "hevc"
	}
	// rc_mode=3 is avbr. rcmode=1 is cbr.
	command := []string{
		"-hide_banner", "-loglevel", "warning", "-nostdin",
		"-fflags", "nobuffer", "-flags", "low_delay",
		"-f", "v4l2", "-input_format", "nv12",
		"-i", device, "-an",
		"-fps_mode", "passthrough", "-c:v", encoder, "-b:v", bitRate, "-maxrate", bitRate, "-bufsize", bitRate, "-rc_mode", "3",
		"-pix_fmt", "nv12",
		"-g", strconv.Itoa(fps * 2), "-bf", "0", "-f", outputFormat, "pipe:1",
	}
	cmd := exec.Command(ffmpeg, command...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create ffmpeg stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		stdout.Close()
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	log.Printf("[rockchip-hdmirx] started ffmpeg: %s %s", ffmpeg, strings.Join(command, " "))
	return &Driver{
		videoChan:   make(chan sdriver.AVBox, 32),
		audioChan:   make(chan sdriver.AVBox),
		controlChan: make(chan sdriver.Event, 1),
		cmd:         cmd,
		stdout:      stdout,
		done:        make(chan struct{}),
		meta: sdriver.MediaMeta{
			VideoCodec: codec,
			Width:      uint32(width),
			Height:     uint32(height),
			FPS:        uint32(fps),
		},
	}, nil
}

func (d *Driver) GetReceivers() (<-chan sdriver.AVBox, <-chan sdriver.AVBox, chan sdriver.Event) {
	return d.videoChan, d.audioChan, d.controlChan
}

func (d *Driver) Start() error {
	d.startOnce.Do(func() { go d.readLoop() })
	return nil
}

func (d *Driver) Pause() {}

func (d *Driver) SendEvent(event sdriver.Event) error {
	return errors.New("rockchip-hdmirx does not support control events")
}

func (d *Driver) RequestIDR(firstFrame bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, nal := range [][]byte{d.lastVPS, d.lastSPS, d.lastPPS, d.lastIDR} {
		if len(nal) > 0 {
			select {
			case d.videoChan <- sdriver.AVBox{Data: append([]byte(nil), nal...), NoDuration: true}:
			default:
				return
			}
		}
	}
}

func (d *Driver) Capabilities() sdriver.DriverCaps {
	return sdriver.DriverCaps{CanVideo: true, IsLinux: true}
}

func (d *Driver) MediaMeta() sdriver.MediaMeta {
	return d.meta
}

func (d *Driver) Stop() {
	d.stopOnce.Do(func() {
		close(d.done)
		_ = d.stdout.Close()
		if d.cmd.Process != nil {
			_ = d.cmd.Process.Kill()
		}
	})
}

func (d *Driver) readLoop() {
	defer close(d.videoChan)
	defer close(d.audioChan)
	defer close(d.controlChan)
	defer func() { _ = d.cmd.Wait() }()

	buffer := make([]byte, 0, 64*1024)
	chunk := make([]byte, 64*1024)
	start := time.Now()
	for {
		n, err := d.stdout.Read(chunk)
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)
			buffer = d.emitNALs(buffer, start)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("[rockchip-hdmirx] ffmpeg output failed: %v", err)
			}
			if len(buffer) > 0 {
				d.emitFinalNAL(buffer, start)
			}
			return
		}
		select {
		case <-d.done:
			return
		default:
		}
	}
}

func (d *Driver) emitFinalNAL(buffer []byte, start time.Time) {
	_, startLen := findStartCode(buffer)
	if startLen == 0 {
		return
	}
	startIndex, _ := findStartCode(buffer)
	d.emitNAL(buffer[startIndex+startLen:], start)
}

func (d *Driver) emitNALs(buffer []byte, start time.Time) []byte {
	for {
		first, firstLen := findStartCode(buffer)
		if first < 0 {
			if len(buffer) > 3 {
				return append([]byte(nil), buffer[len(buffer)-3:]...)
			}
			return buffer
		}
		if first > 0 {
			buffer = buffer[first:]
		}
		next, _ := findStartCode(buffer[firstLen:])
		if next < 0 {
			return buffer
		}
		nal := append([]byte(nil), buffer[firstLen:firstLen+next]...)
		buffer = buffer[firstLen+next:]
		d.emitNAL(nal, start)
	}
}

func (d *Driver) emitNAL(nal []byte, start time.Time) {
	if len(nal) == 0 {
		return
	}
	var nalType byte
	if d.meta.VideoCodec == "h265" {
		if len(nal) < 2 {
			return
		}
		nalType = (nal[0] >> 1) & 0x3f
	} else {
		nalType = nal[0] & 0x1f
	}
	d.mu.Lock()
	isIDR := (d.meta.VideoCodec == "h264" && nalType == 5) ||
		(d.meta.VideoCodec == "h265" && nalType >= 19 && nalType <= 21)
	switch {
	case d.meta.VideoCodec == "h265" && nalType == 32:
		d.lastVPS = append([]byte(nil), nal...)
	case d.meta.VideoCodec == "h265" && nalType == 33:
		d.lastSPS = append([]byte(nil), nal...)
	case d.meta.VideoCodec == "h265" && nalType == 34:
		d.lastPPS = append([]byte(nil), nal...)
	case d.meta.VideoCodec == "h264" && nalType == 7:
		d.lastSPS = append([]byte(nil), nal...)
	case d.meta.VideoCodec == "h264" && nalType == 8:
		d.lastPPS = append([]byte(nil), nal...)
	case isIDR:
		d.lastIDR = append([]byte(nil), nal...)
	}
	d.mu.Unlock()

	data := append([]byte(nil), nal...)
	box := sdriver.AVBox{
		Data:       data,
		PTS:        uint64(time.Since(start).Milliseconds()),
		NoDuration: isConfigNAL(d.meta.VideoCodec, nalType),
	}
	select {
	case d.videoChan <- box:
	case <-d.done:
	}
}

func isConfigNAL(codec string, nalType byte) bool {
	return (codec == "h264" && (nalType == 7 || nalType == 8)) ||
		(codec == "h265" && nalType >= 32 && nalType <= 34)
}

func findStartCode(data []byte) (int, int) {
	for i := 0; i+3 <= len(data); i++ {
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 1 {
			return i, 3
		}
		if i+4 <= len(data) && data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 1 {
			return i, 4
		}
	}
	return -1, 0
}

func parseResolution(value string) (int, int, error) {
	if value == "" {
		value = "1920x1080"
	}
	parts := strings.Split(value, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("resolution must be WIDTHxHEIGHT")
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid resolution width: %w", err)
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid resolution height: %w", err)
	}
	if width < 320 || width > 3840 || height < 240 || height > 2160 {
		return 0, 0, fmt.Errorf("resolution must be between 320x240 and 3840x2160")
	}
	return width, height, nil
}

func parsePositiveInt(value string, fallback int, name string) (int, error) {
	if value == "" {
		return fallback, nil
	}
	result, err := strconv.Atoi(value)
	if err != nil || result < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return result, nil
}

func activeResolution(device string, fallbackWidth, fallbackHeight int) (int, int) {
	output, err := exec.Command("v4l2-ctl", "--get-dv-timings", "--device", device).CombinedOutput()
	if err != nil {
		log.Printf("[rockchip-hdmirx] unable to query active HDMI timings: %v", err)
		return fallbackWidth, fallbackHeight
	}
	var width, height int
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != "Active" {
			continue
		}
		switch fields[1] {
		case "width:":
			width, _ = strconv.Atoi(fields[2])
		case "height:":
			height, _ = strconv.Atoi(fields[2])
		}
	}
	if width > 0 && height > 0 {
		return width, height
	}
	log.Printf("[rockchip-hdmirx] unable to parse active HDMI timings, using %dx%d", fallbackWidth, fallbackHeight)
	return fallbackWidth, fallbackHeight
}

func findDevice(requested string) (string, error) {
	if requested != "" && requested != "auto" {
		if _, err := os.Stat(requested); err != nil {
			return "", fmt.Errorf("HDMI RX device %q is unavailable: %w", requested, err)
		}
		return requested, nil
	}
	entries, err := os.ReadDir("/dev")
	if err != nil {
		return "", fmt.Errorf("scan video devices: %w", err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "video") {
			continue
		}
		path := filepath.Join("/dev", entry.Name())
		name, err := os.ReadFile(filepath.Join("/sys/class/video4linux", entry.Name(), "name"))
		if err == nil && (bytes.Contains(bytes.ToLower(name), []byte("hdmirx")) ||
			bytes.Contains(bytes.ToLower(name), []byte("hdmi rx")) ||
			bytes.Contains(bytes.ToLower(name), []byte("synopsys"))) {
			return path, nil
		}
	}
	return "", errors.New("no HDMI RX V4L2 device found; set driver_config.device to /dev/videoX")
}
