package scrcpy

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"webscreen/utils"
)

func ExecADB(ctx context.Context, args ...string) error {
	adbPath, err := utils.GetADBPath()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, adbPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return err
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
		"video",
		"video_codec",
		"audio",
		"control",
		"new_display",
		"max_size",
		"video_codec_options",
		"log_level",
		"video_encoder",
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

// Global ADB Helper Functions

// GetConnectedDevices returns a list of connected device serials/IPs
func GetConnectedDevices() ([]string, error) {
	adbPath, err := utils.GetADBPath()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(adbPath, "devices")
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
	adbPath, err := utils.GetADBPath()
	if err != nil {
		return err
	}
	cmd := exec.Command(adbPath, "connect", address)
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
	adbPath, err := utils.GetADBPath()
	if err != nil {
		return err
	}
	cmd := exec.Command(adbPath, "pair", address, code)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb pair failed: %v, output: %s", err, string(output))
	}
	if !strings.Contains(string(output), "Successfully paired") {
		return fmt.Errorf("adb pair failed: %s", string(output))
	}
	return nil
}
