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

func (c *ADBClient) StartScrcpyServer(options map[string]string) error {
	cmdStr := toScrcpyCommand(options)
	// 启动 scrcpy-server 是一个阻塞操作，它会一直运行直到连接断开或出错
	// 但我们需要知道它是否成功启动并开始尝试连接
	// 由于 scrcpy-server 的设计，它启动后会尝试连接我们的 reverse tunnel
	// 如果连接失败，它会打印错误并退出
	// 如果连接成功，它会保持运行

	// 我们可以通过检查命令是否立即退出来判断是否启动失败
	// 但更好的方式可能是让调用者通过 Accept 的超时来判断

	// 这里我们改为同步执行，但是在一个 goroutine 中，通过 channel 返回错误
	// 或者，我们可以让它在后台运行，但是捕获它的输出，如果发现错误就报错？

	// 实际上，最简单的办法是：
	// 1. 启动 server (异步)
	// 2. 在主流程中 Accept 连接
	// 3. 如果 Accept 超时，或者 server 进程快速退出，则认为失败

	// 修改为：返回一个 channel，用于接收 server 的退出错误
	errChan := make(chan error, 1)
	go func() {
		time.Sleep(time.Second * 1) // 给一点时间让 reverse tunnel 生效
		log.Printf("Starting scrcpy server with command: %s", cmdStr)
		err := c.Shell(cmdStr)
		if err != nil {
			log.Printf("Scrcpy server exited with error: %v", err)
			errChan <- err
		} else {
			log.Println("Scrcpy server exited normally")
			errChan <- nil
		}
		close(errChan)
	}()

	// 这里我们无法立即知道是否成功，因为 Shell 命令会阻塞
	// 真正的“成功”标志是我们的 Listener Accept 到了连接
	return nil
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
