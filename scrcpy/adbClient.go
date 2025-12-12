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
	Address      string // 设备的IP地址或序列号
	ScrcpyParams ScrcpyParams

	ctx    context.Context
	cancel context.CancelFunc
}

// NewADBClient 创建一个新的 ADB 客户端结构体.
// 如果 address 为空字符串，则表示使用默认设备.
func NewADBClient(address string) *ADBClient {
	defaultScrcpyParams := ScrcpyParams{
		Version:           "3.3.3",
		SCID:              GenerateSCID(),
		MaxFPS:            "60",
		VideoBitRate:      "16000000",
		Control:           "true",
		Audio:             "true",
		VideoCodec:        "h264",
		VideoCodecOptions: "i-frame-interval=1",
		LogLevel:          "debug",
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ADBClient{Address: address, ScrcpyParams: defaultScrcpyParams, ctx: ctx, cancel: cancel}
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

func (c *ADBClient) Adb(args ...string) error {
	// 如果没有指定设备地址，则使用默认 adb 命令
	if c.Address == "" {
		return c.exec_adb(os.Stdout, os.Stderr, args...)
	}
	// 否则，添加 -s 参数指定设备
	return c.exec_adb(os.Stdout, os.Stderr, append([]string{"-s", c.Address}, args...)...)
}

// Shell
func (c *ADBClient) Shell(cmd string) error {
	return c.Adb("shell", cmd)
}

// Push 将本地文件推送到设备上，并更新 ScrcpyParams 中的 CLASSPATH 字段.
func (c *ADBClient) Push(localPath, remotePath string) error {
	err := c.Adb("push", localPath, remotePath)
	if err != nil {
		return fmt.Errorf("ADB Push failed: %v", err)
	}
	c.ScrcpyParams.CLASSPATH = remotePath
	return nil
}

func (c *ADBClient) Reverse(local, remote string) error {
	// c.ReverseRemove(local)
	err := c.Adb("reverse", local, remote)
	if err != nil {
		return fmt.Errorf("ADB Reverse failed: %v", err)
	}
	return nil
}

func (c *ADBClient) ReverseRemove(local string) error {
	err := c.Adb("reverse", "--remove", local)
	if err != nil {
		return fmt.Errorf("ADB Reverse Remove failed: %v", err)
	}
	return nil
}

func (c *ADBClient) StartScrcpyServer() {
	cmdStr := GetScrcpyCommand(c.ScrcpyParams)
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
func ScrcpyParamsToArgs(p ScrcpyParams) []string {
	args := []string{
		// fmt.Sprintf("max_size=%s", p.MaxSize),
		fmt.Sprintf("max_fps=%s", p.MaxFPS),
		fmt.Sprintf("video_bit_rate=%s", p.VideoBitRate),
		fmt.Sprintf("control=%s", p.Control),
		fmt.Sprintf("audio=%s", p.Audio),
		fmt.Sprintf("video_codec=%s", p.VideoCodec),
	}
	if p.MaxSize != "" {
		args = append(args, fmt.Sprintf("max_size=%s", p.MaxSize))
	}
	if p.VideoCodecOptions != "" {
		args = append(args, fmt.Sprintf("video_codec_options=%s", p.VideoCodecOptions))
	}
	args = append(args, fmt.Sprintf("log_level=%s", p.LogLevel))
	return args
}

func GetScrcpyCommand(params ScrcpyParams) string {
	base := fmt.Sprintf("CLASSPATH=%s app_process / com.genymobile.scrcpy.Server %s ",
		params.CLASSPATH, params.Version)
	args := ScrcpyParamsToArgs(params)
	return strings.Join(append([]string{base}, args...), " ")
}
