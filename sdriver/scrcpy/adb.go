package scrcpy

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
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
	c.adb("reverse", "--remove", local)
	// if err != nil {
	// 	return fmt.Errorf("ADB Reverse Remove failed: %v", err)
	// }
	return nil
}

func (c *ADBClient) StartScrcpyServer(options map[string]string) error {
	cmdStr := toScrcpyCommand(options)

	go func() {
		time.Sleep(time.Second * 2) // 给一点时间让 reverse tunnel 生效
		log.Printf("Starting scrcpy server with command: %s", cmdStr)
		err := c.adb("shell", cmdStr)
		if err != nil {
			log.Printf("Failed to run adb shell command: %v", err)
			return
		} else {
			log.Println("Scrcpy server exited normally")
		}
	}()

	// 这里我们无法立即知道是否成功，因为 Shell 命令会阻塞
	// 真正的“成功”标志是我们的 Listener Accept 到了连接
	return nil
}

func (c *ADBClient) adb(args ...string) error {
	// 如果没有指定设备地址，则使用默认 adb 命令
	log.Printf("Executing on device %s: %s", c.deviceSerial, args)
	if c.deviceSerial == "" {
		return ExecADB(c.ctx, args...)
	}
	// 否则，添加 -s 参数指定设备
	return ExecADB(c.ctx, append([]string{"-s", c.deviceSerial}, args...)...)
}

func (c *ADBClient) SupportOpusAudio() bool {
	// 1. 构造 shell 命令
	cmdStr := "grep -i 'opus.encoder' " +
		"/system/etc/media_codecs*.xml " +
		"/system_ext/etc/media_codecs*.xml " +
		"/vendor/etc/media_codecs*.xml " + //常见安卓真机
		"/vendor/odm/etc/media_codecs*.xml " +
		"/odm/etc/media_codecs*.xml " +
		"/product/etc/media_codecs*.xml " +
		"/apex/com.android.media.swcodec/etc/media_codecs*.xml " + //通用
		"/apex/com.android.media/etc/media_codecs*.xml " +
		" 2>/dev/null || true"
	//扩展路径，编码器的参数文件路径不同设备不同，不添加"||true" 会导致返回状态码2无法正常返回stdout

	// 2. 准备 adb 参数
	var args []string
	if c.deviceSerial != "" {
		args = append(args, "-s", c.deviceSerial)
	}
	args = append(args, "shell", cmdStr)

	// 3. 执行命令并捕获输出
	// 使用 c.ctx 以便在父 Context 取消时能够中止命令
	cmd := exec.CommandContext(c.ctx, "adb", args...)

	output, err := cmd.CombinedOutput() // 同时获取 stdout 和 stderr
	if err != nil {
		// 命令执行失败（可能是 adb 没连接，或者 app_process 报错）
		log.Printf("Failed to check audio encoders: %v", err)
		return false
	}

	// 4. 检查输出中是否包含 "opus.encoder"
	outputStr := string(output)

	// 调试日志：可选，查看设备实际返回了什么
	log.Printf("opus Encoder : %s", outputStr)

	return strings.Contains(outputStr, "opus.encoder")
}
