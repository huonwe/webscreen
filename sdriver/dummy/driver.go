package dummy

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"webcpy/sdriver"
	"webcpy/sdriver/comm"

	"github.com/pion/webrtc/v4/pkg/media/h264reader"
	"github.com/pion/webrtc/v4/pkg/media/h265reader"
)

// DummyDriver implements sdriver.SDriver by reading from a local H.264 Annex B file.
type DummyDriver struct {
	filePath string

	mediaMeta sdriver.MediaMeta

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
func New(c sdriver.StreamConfig) (*DummyDriver, error) {
	d := &DummyDriver{
		// filePath:  c.OtherOpts["file"],
		// fps:       c.OtherOpts["fps"],
		stopCh:    make(chan struct{}),
		videoCh:   make(chan sdriver.AVBox, 64),
		audioCh:   make(chan sdriver.AVBox, 1),
		controlCh: make(chan sdriver.ControlEvent, 1),
	}
	if c.OtherOpts != nil {
		if v, ok := c.OtherOpts["file_path"]; ok {
			d.filePath = v
		}
	}
	if d.filePath == "" {
		return nil, errors.New("dummy: file path is empty")
	}

	d.fetchMediaMeta()
	log.Printf("Dummy driver media meta: %+v", d.mediaMeta)

	return d, nil
}

// GetReceiver returns the channels for video, audio, and control events.
func (d *DummyDriver) GetReceivers() (<-chan sdriver.AVBox, <-chan sdriver.AVBox, <-chan sdriver.ControlEvent) {
	return d.videoCh, d.audioCh, d.controlCh
}

// StartStream starts reading the H.264 file and produces AVBox packets.
func (d *DummyDriver) StartStreaming() {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	select {
	case <-d.stopCh:
		d.mu.Unlock()
		return
	default:
	}
	d.running = true
	d.mu.Unlock()
	go d.loop()
}

func (d *DummyDriver) StopStreaming() {
	log.Println("DummyDriver: StopStreaming called")
	d.Stop()
}

// SendEvent is a no-op for dummy driver.
func (d *DummyDriver) SendEvent(event sdriver.Event) error { return nil }

// RequestIDR attempts to resend cached SPS/PPS instantly (if available).
func (d *DummyDriver) RequestIDR() {
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
}

// Capabilities reports what this driver supports.
func (d *DummyDriver) Capabilities() sdriver.DriverCaps {
	return sdriver.DriverCaps{CanClipboard: false, CanUHID: false, CanVideo: true, CanAudio: false, CanControl: false}
}

// CodecInfo returns the video/audio codec identifiers.
func (d *DummyDriver) CodecInfo() (string, string) {
	if d.mediaMeta.VideoCodecID != "" {
		return d.mediaMeta.VideoCodecID, ""
	}
	return "h264", ""
}

func (d *DummyDriver) MediaMeta() sdriver.MediaMeta {
	return d.mediaMeta
}

// Stop stops the streaming loop and closes channels.
func (d *DummyDriver) Stop() {
	d.stopOnce.Do(func() {
		close(d.stopCh)
		// Allow the loop to close the output channels
	})
}

func (d *DummyDriver) loop() {
	defer func() {
		d.mu.Lock()
		d.running = false
		d.mu.Unlock()
		close(d.videoCh)
		close(d.audioCh)
		close(d.controlCh)
	}()

	// 设定固定的帧间隔，例如 30fps = 33.3ms
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	for {
		// 1. 打开文件
		f, err := os.Open(d.filePath)
		if err != nil {
			fmt.Println("File open error:", err)
			return
		}

		var nalReader interface {
			NextNAL() ([]byte, error)
		}

		if d.mediaMeta.VideoCodecID == "h265" {
			h265, err := h265reader.NewReader(f)
			if err != nil {
				log.Printf("Failed to create H.265 reader: %v", err)
				f.Close()
				return
			}
			nalReader = &h265ReaderWrapper{h265}
		} else {
			h264, err := h264reader.NewReader(f)
			if err != nil {
				f.Close()
				return
			}
			nalReader = &h264ReaderWrapper{h264}
		}

		// 3. 读取 NAL 循环
		for {
			// 这一步会自动处理 Annex-B 的分割
			nalData, err := nalReader.NextNAL()
			if err == io.EOF {
				break // 文件读完了，跳出内层循环，重新开始外层循环（Loop）
			}
			if err != nil {
				break
			}

			// 检查外部停止信号
			select {
			case <-d.stopCh:
				f.Close()
				return
			default:
			}

			var isIDR, isConfig, isVCL bool

			if d.mediaMeta.VideoCodecID == "h265" {
				// H.265 NAL header parsing
				// Forbidden_zero_bit (1) + Nal_unit_type (6) + Nuh_layer_id (6) + Nuh_temporal_id_plus1 (3)
				// nal_unit_type is bits 1-6 of the first byte
				nalType := (nalData[0] >> 1) & 0x3F

				// VPS(32), SPS(33), PPS(34)
				isConfig = nalType >= 32 && nalType <= 34
				// IDR_W_RADL(19), IDR_N_LP(20), CRA_NUT(21)
				isIDR = nalType >= 19 && nalType <= 21
				// VCL NAL units are 0-31
				isVCL = nalType < 32
			} else {
				// H.264 NAL header parsing
				nalType := nalData[0] & 0x1F
				isIDR = nalType == 5
				isConfig = nalType == 7 || nalType == 8
				isVCL = !isConfig && nalType != 6 // Exclude SEI
			}

			// 你的业务逻辑：存储 SPS/PPS
			if isConfig {
				d.mu.Lock()
				// For simplicity, we just store the last config frame as SPS/PPS equivalent
				// In a real H.265 implementation, you'd want to store VPS/SPS/PPS separately
				if d.mediaMeta.VideoCodecID == "h265" {
					// Simple storage for H.265 config frames
					d.lastSPS = nalData // Just store as lastSPS for now to satisfy RequestIDR
				} else {
					if (nalData[0] & 0x1F) == 7 {
						d.lastSPS = nalData
					} else {
						d.lastPPS = nalData
					}
				}
				d.mu.Unlock()
			}

			// 发送数据
			// 这里我们先不计算 PTS，交给 Agent 去累加，或者在这里简单处理
			box := sdriver.AVBox{
				Data:       nalData,
				PTS:        0, // 这里填 0，由 Agent 根据接收频率或固定帧率计算
				IsKeyFrame: isIDR,
				IsConfig:   isConfig,
			}

			select {
			case d.videoCh <- box:
			case <-d.stopCh:
				f.Close()
				return
			}

			// --- 核心节奏控制 ---
			if isVCL {
				<-ticker.C
				// ⚠️ 注意：如果你的文件是多 Slice (一个帧由多个 NAL 组成)
				// 这里会导致严重的慢动作和抖动！
				// 必须配合我上一条回答里的 ffmpeg -slices 1 命令使用
			}
		}

		f.Close()
		fmt.Println("Looping video...")
	}
}

type h264ReaderWrapper struct {
	*h264reader.H264Reader
}

func (r *h264ReaderWrapper) NextNAL() ([]byte, error) {
	nal, err := r.H264Reader.NextNAL()
	if err != nil {
		return nil, err
	}
	return nal.Data, nil
}

type h265ReaderWrapper struct {
	*h265reader.H265Reader
}

func (r *h265ReaderWrapper) NextNAL() ([]byte, error) {
	nal, err := r.H265Reader.NextNAL()
	if err != nil {
		return nil, err
	}
	return nal.Data, nil
}

func (d *DummyDriver) fetchMediaMeta() {
	// Try to parse SPS to get resolution
	if f, err := os.Open(d.filePath); err == nil {
		// Read first 4KB, should be enough for SPS
		buf := make([]byte, 4096)
		n, _ := f.Read(buf)
		f.Close()

		if n > 0 {
			nals := splitAnnexB(buf[:n])
			for _, nal := range nals {
				if len(nal) > 0 && (nal[0]&0x1F) == 7 {
					// Found SPS
					// Remove emulation prevention bytes
					rbsp := comm.RemoveEmulationPreventionBytes(nal)
					spsInfo, err := comm.ParseSPS_H264(rbsp, false)
					if err == nil {
						d.mediaMeta.Width = spsInfo.Width
						d.mediaMeta.Height = spsInfo.Height
						d.mediaMeta.VideoCodecID = "h264"
						if spsInfo.FrameRate > 0 {
							d.mediaMeta.FPS = uint32(spsInfo.FrameRate + 0.5)
						}
					}
					break
				}
			}
		}
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

// func parseFPS(s string) (int, error) {
// 	// very small helper: accept simple integer
// 	var v int
// 	for _, ch := range s {
// 		if ch < '0' || ch > '9' {
// 			return 0, errors.New("invalid fps")
// 		}
// 		v = v*10 + int(ch-'0')
// 	}
// 	if v <= 0 {
// 		return 0, errors.New("invalid fps")
// 	}
// 	return v, nil
// }
