package scrcpy

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ADBClient struct {
	deviceSerial string // 设备的IP地址或序列号
	ctx          context.Context
	cancel       context.CancelFunc
}

func ExecADB(args ...string) error {
	cmd := exec.Command("adb", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// NewClient 创建一个新的 ADB 客户端结构体.
// 如果 address 为空字符串，则表示使用默认设备.
func NewADBClient(deviceSerial string) *ADBClient {

	ctx, cancel := context.WithCancel(context.Background())
	return &ADBClient{deviceSerial: deviceSerial, ctx: ctx, cancel: cancel}
}

// 显式停止服务的方法
func (c *ADBClient) Stop() {
	c.cancel() // 这会触发所有绑定了该 ctx 的命令被 Kill
}

func (c *ADBClient) exec_adb(stdout, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(c.ctx, "adb", args...)
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}

// Shell
func (c *ADBClient) Shell(cmd string) error {
	return c.adb("shell", cmd)
}

// Push 将本地文件推送到设备上
func (c *ADBClient) Push(localPath string, remotePath string) error {
	if remotePath == "" {
		remotePath = "/data/local/tmp/scrcpy-server"
	}
	err := c.adb("push", localPath, remotePath)
	if err != nil {
		return fmt.Errorf("ADB Push failed: %v", err)
	}
	// c.ScrcpyParams.CLASSPATH = remotePath
	return nil
}

func (c *ADBClient) Reverse(local, remote string) error {
	// c.ReverseRemove(local)
	err := c.adb("reverse", local, remote)
	if err != nil {
		return fmt.Errorf("ADB Reverse failed: %v", err)
	}
	return nil
}

func (c *ADBClient) ReverseRemove(local string) error {
	err := c.adb("reverse", "--remove", local)
	if err != nil {
		return fmt.Errorf("ADB Reverse Remove failed: %v", err)
	}
	return nil
}

func (c *ADBClient) StartScrcpyServer(options map[string]string) {
	cmdStr := toScrcpyCommand(options)
	log.Printf("cmdStr: %s", cmdStr)
	go func() {
		err := c.Shell(cmdStr)
		if err != nil {
			log.Printf("Failed to start scrcpy server: %v", err)
		}
	}()
}

func GenerateSCID() string {
	seed := time.Now().UnixNano() + rand.Int63()
	r := rand.New(rand.NewSource(seed))
	// 生成31位随机整数
	return strconv.FormatInt(int64(r.Uint32()&0x7FFFFFFF), 16)
}

// 将ScrcpyParams转为 key=value 格式的参数列表
func scrcpyParamsToArgs(params map[string]string) []string {
	var args []string
	keys := []string{
		"scid",
		"max_fps",
		"video_bit_rate",
		"control",
		"audio",
		"video_codec",
		"new_display",
		"max_size",
		"video_codec_options",
		"log_level",
	}
	for _, key := range keys {
		if v, ok := params[key]; ok && v != "" {
			args = append(args, fmt.Sprintf("%s=%s", key, v))
		}
	}
	return args
}

func toScrcpyCommand(options map[string]string) string {
	classpath := options["CLASSPATH"]
	version := options["Version"]
	base := fmt.Sprintf("CLASSPATH=%s app_process / com.genymobile.scrcpy.Server %s ",
		classpath, version)
	args := scrcpyParamsToArgs(options)
	return strings.Join(append([]string{base}, args...), " ")
}

func (c *ADBClient) adb(args ...string) error {
	// 如果没有指定设备地址，则使用默认 adb 命令
	log.Printf("Executing on device %s: %s", c.deviceSerial, args)
	if c.deviceSerial == "" {
		return c.exec_adb(os.Stdout, os.Stderr, args...)
	}
	// 否则，添加 -s 参数指定设备
	return c.exec_adb(os.Stdout, os.Stderr, append([]string{"-s", c.deviceSerial}, args...)...)
}

// Global ADB Helper Functions

// GetConnectedDevices returns a list of connected device serials/IPs
func GetConnectedDevices() ([]string, error) {
	cmd := exec.Command("adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var devices []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "List of devices attached") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == "device" {
			devices = append(devices, parts[0])
		}
	}
	return devices, nil
}

// ConnectDevice connects to a device via TCP/IP
func ConnectDevice(address string) error {
	cmd := exec.Command("adb", "connect", address)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb connect failed: %v, output: %s", err, string(output))
	}
	if strings.Contains(string(output), "unable to connect") || strings.Contains(string(output), "failed to connect") {
		return fmt.Errorf("adb connect failed: %s", string(output))
	}
	return nil
}

// PairDevice pairs with a device using a pairing code
func PairDevice(address, code string) error {
	cmd := exec.Command("adb", "pair", address, code)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb pair failed: %v, output: %s", err, string(output))
	}
	if !strings.Contains(string(output), "Successfully paired") {
		return fmt.Errorf("adb pair failed: %s", string(output))
	}
	return nil
}
