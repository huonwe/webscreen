package adb

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"webcpy/scrcpy"
)

type Client struct {
	Address      string // 设备的IP地址或序列号
	ScrcpyParams scrcpy.ScrcpyParams

	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient 创建一个新的 ADB 客户端结构体.
// 如果 address 为空字符串，则表示使用默认设备.
func NewClient(address string) *Client {
	defaultScrcpyParams := scrcpy.ScrcpyParams{
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
	return &Client{Address: address, ScrcpyParams: defaultScrcpyParams, ctx: ctx, cancel: cancel}
}

// 显式停止服务的方法
func (c *Client) Stop() {
	c.cancel() // 这会触发所有绑定了该 ctx 的命令被 Kill
}

func (c *Client) exec_adb(stdout, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(c.ctx, "adb", args...)
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}

func (c *Client) Adb(args ...string) error {
	// 如果没有指定设备地址，则使用默认 adb 命令
	if c.Address == "" {
		return c.exec_adb(os.Stdout, os.Stderr, args...)
	}
	// 否则，添加 -s 参数指定设备
	return c.exec_adb(os.Stdout, os.Stderr, append([]string{"-s", c.Address}, args...)...)
}

// Shell
func (c *Client) Shell(cmd string) error {
	return c.Adb("shell", cmd)
}

// Push 将本地文件推送到设备上，并更新 ScrcpyParams 中的 CLASSPATH 字段.
func (c *Client) Push(localPath, remotePath string) error {
	err := c.Adb("push", localPath, remotePath)
	if err != nil {
		return fmt.Errorf("ADB Push failed: %v", err)
	}
	c.ScrcpyParams.CLASSPATH = remotePath
	return nil
}

func (c *Client) Reverse(local, remote string) error {
	// c.ReverseRemove(local)
	err := c.Adb("reverse", local, remote)
	if err != nil {
		return fmt.Errorf("ADB Reverse failed: %v", err)
	}
	return nil
}

func (c *Client) ReverseRemove(local string) error {
	err := c.Adb("reverse", "--remove", local)
	if err != nil {
		return fmt.Errorf("ADB Reverse Remove failed: %v", err)
	}
	return nil
}

func (c *Client) StartScrcpyServer() {
	cmdStr := GetScrcpyCommand(c.ScrcpyParams)
	log.Printf("cmdStr: %s", cmdStr)
	go func() {
		err := c.Shell(cmdStr)
		if err != nil {
			log.Printf("Failed to start scrcpy server: %v", err)
		}
	}()
}
