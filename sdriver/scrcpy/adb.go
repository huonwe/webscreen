package scrcpy

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

type ADBClient struct {
	deviceSerial string
	scid         string
	remotePath   string
	ctx          context.Context
	cancel       context.CancelFunc
	transport    ADBTransport
	transportErr error
}

func NewADBClient(deviceSerial string, scid string, parentCtx context.Context) *ADBClient {
	ctx, cancel := context.WithCancel(parentCtx)
	transport, err := NewADBTransportFromEnvironment()
	return &ADBClient{
		deviceSerial: deviceSerial,
		scid:         scid,
		ctx:          ctx,
		cancel:       cancel,
		transport:    transport,
		transportErr: err,
	}
}

func (c *ADBClient) Stop() {
	if c.scid != "" && c.transportErr == nil && c.transport != nil && c.transport.Capabilities().CanReverse {
		_ = c.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", c.scid))
	}
	c.cancel()
}

func (c *ADBClient) PushScrcpyServer(localPath string, remotePath string) error {
	if c.remotePath == "" {
		c.remotePath = "/data/local/tmp/scrcpy-server"
	}
	if c.transportErr != nil {
		return c.transportErr
	}
	if err := c.transport.Push(c.ctx, c.deviceSerial, localPath, c.remotePath); err != nil {
		return fmt.Errorf("ADB Push failed: %v", err)
	}
	return nil
}

func (c *ADBClient) Reverse(remote, local string) error {
	if c.transportErr != nil {
		return c.transportErr
	}
	if err := c.transport.Reverse(c.ctx, c.deviceSerial, remote, local); err != nil {
		return fmt.Errorf("ADB Reverse failed: %v", err)
	}
	return nil
}

func (c *ADBClient) ReverseRemove(remote string) error {
	if c.transportErr != nil || c.transport == nil {
		return c.transportErr
	}
	_ = c.transport.ReverseRemove(c.ctx, c.deviceSerial, remote)
	return nil
}

func (c *ADBClient) StartScrcpyServer(options map[string]string) error {
	cmdStr := toScrcpyCommand(options)

	go func() {
		time.Sleep(2 * time.Second)
		log.Printf("Starting scrcpy server with command: %s", cmdStr)
		if c.transportErr != nil {
			log.Printf("Failed to initialize adb transport: %v", c.transportErr)
			return
		}
		if _, err := c.transport.Shell(c.ctx, c.deviceSerial, cmdStr); err != nil {
			log.Printf("Failed to run adb shell command: %v", err)
			return
		}
		log.Println("Scrcpy server exited normally")
	}()

	return nil
}

func (c *ADBClient) SupportOpusAudio() bool {
	cmdStr := "grep -i 'opus.encoder' " +
		"/system/etc/media_codecs*.xml " +
		"/system_ext/etc/media_codecs*.xml " +
		"/vendor/etc/media_codecs*.xml " +
		"/vendor/odm/etc/media_codecs*.xml " +
		"/odm/etc/media_codecs*.xml " +
		"/product/etc/media_codecs*.xml " +
		"/apex/com.android.media.swcodec/etc/media_codecs*.xml " +
		"/apex/com.android.media/etc/media_codecs*.xml " +
		" 2>/dev/null || true"

	if c.transportErr != nil {
		log.Printf("Failed to initialize adb transport: %v", c.transportErr)
		return false
	}
	output, err := c.transport.Shell(c.ctx, c.deviceSerial, cmdStr)
	if err != nil {
		log.Printf("Failed to check audio encoders: %v", err)
		return false
	}

	outputStr := string(output)
	log.Printf("opus Encoder : %s", outputStr)
	return strings.Contains(outputStr, "opus.encoder")
}

func (c *ADBClient) SupportedEncoderList() []string {
	cmdStr := "grep -E '<MediaCodec name=\"[^\"]*encoder[^\"]*\"' " +
		"/system/etc/media_codecs*.xml " +
		"/system_ext/etc/media_codecs*.xml " +
		"/vendor/etc/media_codecs*.xml " +
		"/vendor/odm/etc/media_codecs*.xml " +
		"/odm/etc/media_codecs*.xml " +
		"/product/etc/media_codecs*.xml " +
		"/apex/com.android.media.swcodec/etc/media_codecs*.xml " +
		"/apex/com.android.media/etc/media_codecs*.xml " +
		" 2>/dev/null || true"

	if c.transportErr != nil {
		log.Printf("Failed to initialize adb transport: %v", c.transportErr)
		return nil
	}
	output, err := c.transport.Shell(c.ctx, c.deviceSerial, cmdStr)
	if err != nil {
		log.Printf("Failed to get supported encoders: %v", err)
		return nil
	}

	var encoders []string
	for _, line := range strings.Split(string(output), "\n") {
		if idx := strings.Index(line, "name=\""); idx != -1 {
			start := idx + len("name=\"")
			if end := strings.Index(line[start:], "\""); end != -1 {
				name := line[start : start+end]
				if strings.Contains(strings.ToLower(name), "encoder") && !containsString(encoders, name) {
					encoders = append(encoders, name)
				}
			}
		}
	}
	return encoders
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
