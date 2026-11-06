package scrcpy

// This file implements the "on-device" (root, no adb) transport.
//
// The regular path drives a device from a host: adb push + adb reverse + a
// local TCP listener. On the device itself there is no adb, but we do not
// need one either:
//
//   - scrcpy-server is started locally with app_process (webscreen is already
//     running as root there, the server drops back to the shell uid itself);
//   - the server is asked for tunnel_forward=true, which makes it create the
//     abstract unix socket "scrcpy_<scid>" and accept the three connections
//     (video, audio, control) on it;
//   - we simply connect to that abstract socket three times.
//
// Everything after the three connections (device metadata, frame framing,
// WebRTC, control, clipboard, uhid) is shared with the adb based path.

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// LOCAL_WORK_DIR is a writable and executable location on the device.
	LOCAL_WORK_DIR = "/data/local/tmp/webscreen"
	// LOCAL_SERVER_PATH is where scrcpy-server is stored on the device.
	LOCAL_SERVER_PATH = LOCAL_WORK_DIR + "/scrcpy-server"
	// UHID_DEVICE is the uinput/hid device scrcpy uses for virtual input.
	UHID_DEVICE = "/dev/uhid"
	// AID_UHID is the Android uid/gid owning the uhid device group.
	AID_UHID = 3011

	// LOCAL_CONNECT_TIMEOUT is how long we wait for the scrcpy server to
	// create its listener.
	LOCAL_CONNECT_TIMEOUT = 15 * time.Second
)

// localServerGroups and the SysProcAttr used to keep the uhid group live in
// platform specific files (local_groups_unix.go / local_groups_other.go):
// syscall.Credential and os.Getgroups only exist on unix, and the on-device
// transport is only meaningful on Linux/Android anyway.

var (
	localEncodersOnce sync.Once
	localEncoders     []string
	localNameOnce     sync.Once
	localName         string
)

// writeLocalScrcpyServer stores the embedded scrcpy-server on the device and
// returns the absolute path to use as CLASSPATH.
func writeLocalScrcpyServer(data []byte) (string, error) {
	if err := os.MkdirAll(LOCAL_WORK_DIR, 0755); err != nil {
		return "", fmt.Errorf("create %s failed: %v", LOCAL_WORK_DIR, err)
	}
	if err := os.WriteFile(LOCAL_SERVER_PATH, data, 0644); err != nil {
		return "", fmt.Errorf("write %s failed: %v", LOCAL_SERVER_PATH, err)
	}
	return LOCAL_SERVER_PATH, nil
}

// abstractListener mimics net.Listener on top of the abstract unix socket
// created by the scrcpy server.
//
// The scrcpy server accepts every stream connection (video, audio, control)
// *before* it starts writing the device metadata, so all connections must be
// established up front: connecting one and reading from it first would
// deadlock. All connections are therefore dialed in order in a background
// goroutine and handed out by Accept().
type abstractListener struct {
	name      string
	timeout   time.Duration
	count     int
	conns     chan net.Conn
	startOnce sync.Once
	closed    bool
}

func newAbstractListener(name string, timeout time.Duration, count int) net.Listener {
	if count < 1 {
		count = 1
	}
	return &abstractListener{
		name:    name,
		timeout: timeout,
		count:   count,
		conns:   make(chan net.Conn, count),
	}
}

func (l *abstractListener) socketName() string {
	// Go maps a leading '@' to the abstract namespace NUL byte.
	return "@" + l.name
}

func (l *abstractListener) Accept() (net.Conn, error) {
	l.startOnce.Do(func() { go l.connectAll() })
	select {
	case conn := <-l.conns:
		if conn == nil {
			return nil, fmt.Errorf("could not connect to abstract socket %s", l.name)
		}
		return conn, nil
	case <-time.After(l.timeout):
		return nil, fmt.Errorf("timeout connecting to abstract socket %s", l.name)
	}
}

func (l *abstractListener) connectAll() {
	deadline := time.Now().Add(l.timeout)
	var lastErr error
	for i := 0; i < l.count; i++ {
		for {
			if l.closed {
				l.conns <- nil
				return
			}
			conn, err := net.Dial("unix", l.socketName())
			if err == nil {
				l.conns <- conn
				break
			}
			lastErr = err
			if time.Now().After(deadline) {
				log.Printf("[scrcpy] connect %d/%d to %s failed: %v", i+1, l.count, l.name, lastErr)
				l.conns <- nil
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	log.Printf("[scrcpy] connected %d stream socket(s) to %s", l.count, l.name)
}

func (l *abstractListener) Close() error {
	l.closed = true
	return nil
}

func (l *abstractListener) Addr() net.Addr {
	return &net.UnixAddr{Name: l.socketName(), Net: "unix"}
}

// toScrcpyArgv builds the argv used to start the scrcpy server locally. It is
// the argv equivalent of toScrcpyCommand (which is used over adb shell).
func toScrcpyArgv(options map[string]string) []string {
	version := options["Version"]
	args := []string{"/", "com.genymobile.scrcpy.Server", version}
	return append(args, scrcpyParamsToArgs(options)...)
}

// appProcessPath returns the app_process binary of the current abi.
func appProcessPath() string {
	if _, err := os.Stat("/system/bin/app_process64"); err == nil {
		return "/system/bin/app_process64"
	}
	return "/system/bin/app_process"
}

// shellQuote single-quotes a value for /system/bin/sh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// startLocalScrcpyServer runs the scrcpy server on this device.
//
// It is started through /system/bin/sh on purpose: app_process resolves the
// main class from the classpath environment prepared by the shell. Executing
// app_process directly from the Go process makes ART abort with
// ClassNotFoundException: com.genymobile.scrcpy.Server (verified on Android 16).
func (da *ScrcpyDriver) startLocalScrcpyServer(options map[string]string) error {
	classpath := options["CLASSPATH"]
	argv := toScrcpyArgv(options)
	for i, a := range argv {
		argv[i] = shellQuote(a)
	}
	cmdStr := fmt.Sprintf("CLASSPATH=%s ANDROID_ROOT=/system ANDROID_DATA=/data TMPDIR=%s %s %s",
		shellQuote(classpath), LOCAL_WORK_DIR, appProcessPath(), strings.Join(argv, " "))
	log.Printf("[scrcpy] starting local scrcpy server: %s", cmdStr)

	cmd := exec.CommandContext(da.ctx, "/system/bin/sh", "-c", cmdStr)
	cmd.Env = os.Environ()
	cmd.Dir = LOCAL_WORK_DIR
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if os.Geteuid() == 0 {
		// Keep the uhid group (see localServerGroups) so that the virtual input
		// devices can be created once scrcpy dropped to the shell uid.
		cmd.SysProcAttr = localSysProcAttr(localServerGroups())
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start scrcpy server failed: %v", err)
	}
	da.localServerCmd = cmd

	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("[scrcpy] local scrcpy server exited: %v", err)
		} else {
			log.Println("[scrcpy] local scrcpy server exited normally")
		}
		da.markDead("scrcpy server 已退出")
	}()

	return nil
}

// stopLocalServer stops the locally started scrcpy server.
func (da *ScrcpyDriver) stopLocalServer() {
	if da.localServerCmd != nil && da.localServerCmd.Process != nil {
		if err := da.localServerCmd.Process.Kill(); err != nil {
			log.Printf("[scrcpy] kill scrcpy server failed: %v", err)
		}
	}
	// Deliberately no blanket `pkill -f com.genymobile.scrcpy.Server` here: the
	// agent may be rebuilt right away (dead pipeline or changed settings) and an
	// asynchronous pkill would kill the freshly started server as well.
	// The app kills leftover servers itself when the whole service is stopped.
}

// cleanupTunnel releases the transport resources of the current session.
func (da *ScrcpyDriver) cleanupTunnel() {
	if da.localMode {
		da.stopLocalServer()
		return
	}
	da.adbClient.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", da.scid))
}

// supportOpusAudio reports whether the device can encode opus audio.
func (da *ScrcpyDriver) supportOpusAudio() bool {
	if da.localMode {
		return LocalSupportsOpusAudio()
	}
	return da.adbClient.SupportOpusAudio()
}

// ---------------------------------------------------------------------------
// Local codec probing (the adb path runs `adb shell grep` on the same files)
// ---------------------------------------------------------------------------

func localCodecXMLPaths() []string {
	patterns := []string{
		"/system/etc/media_codecs*.xml",
		"/system_ext/etc/media_codecs*.xml",
		"/vendor/etc/media_codecs*.xml",
		"/vendor/odm/etc/media_codecs*.xml",
		"/odm/etc/media_codecs*.xml",
		"/product/etc/media_codecs*.xml",
		"/apex/com.android.media.swcodec/etc/media_codecs*.xml",
		"/apex/com.android.media/etc/media_codecs*.xml",
	}
	var paths []string
	for _, pattern := range patterns {
		if matches, err := filepath.Glob(pattern); err == nil {
			paths = append(paths, matches...)
		}
	}
	return paths
}

func readLocalCodecXML() string {
	var sb strings.Builder
	for _, path := range localCodecXMLPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sb.Write(data)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// LocalSupportsOpusAudio reports whether the device exposes an opus encoder.
func LocalSupportsOpusAudio() bool {
	return strings.Contains(readLocalCodecXML(), "opus.encoder")
}

// LocalVideoEncoderList parses the on-device media codec configuration files
// and returns the available video encoders.
func LocalVideoEncoderList() []string {
	localEncodersOnce.Do(func() {
		content := readLocalCodecXML()
		// Same rule as the adb based path: only <MediaCodec> entries whose name
		// contains "encoder" and which are video codecs. This keeps non-encoder
		// entries (e.g. <Feature name="max-video-encoder-input-buffers">) out of
		// the list, they can never be used as a video_encoder.
		nameRe := regexp.MustCompile(`<MediaCodec name="([^"]*encoder[^"]*)"`)
		for _, line := range strings.Split(content, "\n") {
			if !strings.Contains(strings.ToLower(line), "video/") {
				continue
			}
			m := nameRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			name := m[1]
			dup := false
			for _, e := range localEncoders {
				if e == name {
					dup = true
					break
				}
			}
			if !dup {
				localEncoders = append(localEncoders, name)
			}
		}
		if localEncoders == nil {
			localEncoders = []string{}
		}
		log.Printf("[scrcpy] local video encoders: %v", localEncoders)
	})
	return localEncoders
}

// LocalDeviceName returns a human readable name for this device, used to
// identify the on-device entry in the device list.
func LocalDeviceName() string {
	localNameOnce.Do(func() {
		out, err := exec.Command("/system/bin/getprop", "ro.product.model").Output()
		if err == nil {
			localName = strings.TrimSpace(string(out))
		}
		if localName == "" {
			localName = "local"
		}
	})
	return localName
}
