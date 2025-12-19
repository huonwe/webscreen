package scrcpy

import (
	"context"
	"fmt"
	"log"
	"time"
)

type ADBClient struct {
	deviceSerial string // 设备的IP地址或序列号
	scid         string
	remotePath   string
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewClient 创建一个新的 ADB 客户端结构体.
// 如果 address 为空字符串，则表示使用默认设备.
func NewADBClient(deviceSerial string, scid string, parentCtx context.Context) *ADBClient {
	ctx, cancel := context.WithCancel(parentCtx)
	return &ADBClient{deviceSerial: deviceSerial, scid: scid, ctx: ctx, cancel: cancel}
}

// 显式停止服务的方法
func (c *ADBClient) Stop() {
	c.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", c.scid))
	c.cancel() // 这会触发所有绑定了该 ctx 的命令被 Kill
}

// Shell
func (c *ADBClient) Shell(cmd string) error {
	return c.adb("shell", cmd)
}

// Push 将本地文件推送到设备上
func (c *ADBClient) PushScrcpyServer(localPath string, remotePath string) error {
	if c.remotePath == "" {
		c.remotePath = "/data/local/tmp/scrcpy-server"
	}
	err := c.adb("push", localPath, c.remotePath)
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
	go func() {
		time.Sleep(time.Second * 1)
		err := c.Shell(cmdStr)
		if err != nil {
			log.Printf("Failed to start scrcpy server: %v", err)
			//c.ReverseRemove("localabstract:scrcpy_123")
		}
	}()
}

func (c *ADBClient) adb(args ...string) error {
	// 如果没有指定设备地址，则使用默认 adb 命令
	log.Printf("Executing on device %s: %s", c.deviceSerial, args)
	if c.deviceSerial == "" {
		return ExecADB(args...)
	}
	// 否则，添加 -s 参数指定设备
	return ExecADB(append([]string{"-s", c.deviceSerial}, args...)...)
}
