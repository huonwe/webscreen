package webservice

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"webscreen/utils"
)

// 受控端背光控制（隐私保护：屏幕黑、但不锁屏、串流与触控照常）。
//
// 为什么不按电源键：KEYCODE_POWER 会让设备熄屏并**锁定**，恢复时需要解锁，
// 与"串流仍可操作"的目标冲突。这里只把 backlight 的 brightness 写 0：
// 面板停止发光，设备保持唤醒状态，触摸/按键/串流全部正常。
//
// 关闭前会记录每个背光节点的原值与 max，恢复到文件里；恢复优先写回原值，
// 原值不可用时退回 max_brightness。
const backlightOffScript = `
set -e
prev=/data/local/tmp/webscreen_backlight_prev
: > "$prev"
for d in /sys/class/backlight/*/; do
  f="${d}brightness"
  [ -w "$f" ] || continue
  cur=$(cat "$f" 2>/dev/null || echo 0)
  max=$(cat "${d}max_brightness" 2>/dev/null || echo 0)
  [ "$cur" -gt 0 ] 2>/dev/null || continue
  echo "$d $cur $max" >> "$prev"
  echo 0 > "$f"
done
echo WEBSREEN_BACKLIGHT_OK
`

const backlightOnScript = `
set -e
prev=/data/local/tmp/webscreen_backlight_prev
if [ -s "$prev" ]; then
  # 每行格式： <目录> <原亮度> <最大亮度>；优先写回原亮度，缺失才用最大值。
  # 注意用 set -- 显式取字段，避免 read 在字段数不符时把值错位。
  while read -r line; do
    set -- $line
    d=$1; cur=${2:-}; max=${3:-}
    f="${d}brightness"
    [ -w "$f" ] || continue
    v=$cur
    case "$v" in ''|*[!0-9]*) v= ;;
    esac
    if [ -z "$v" ]; then
      v=$max
      case "$v" in ''|*[!0-9]*) v=255 ;;
      esac
    fi
    echo "$v" > "$f"
  done < "$prev"
  : > "$prev"
else
  for d in /sys/class/backlight/*/; do
    f="${d}brightness"
    [ -w "$f" ] || continue
    max=$(cat "${d}max_brightness" 2>/dev/null || echo 0)
    [ "$max" -gt 0 ] 2>/dev/null && echo "$max" > "$f"
  done
fi
echo WEBSREEN_BACKLIGHT_OK
`

type backlightRequest struct {
	Serial string `json:"serial"`
	Off    *bool  `json:"off"`
}

// resolveSerial 把页面传来的设备标识解析成 adb 能识别的裸串号。
//
// 串流页面 URL 里的设备 ID 形态并不统一（实测遇到过三种）：
//
//	10fda345                 纯 adb 串号
//	10fda345_0_0             scrcpy 三段式 <serial>_<usbIndex>_<transportId>
//	android_10fda345         <deviceType>_<serial>
//
// 逐个穷举格式既脆弱又容易漏（修了 `_0_0` 又蹦出 `android_` 前缀），
// 因此改为拿 `adb devices` 的在线列表做匹配，顺序：
//  1. 原样命中
//  2. 按 "_" 分段，任一段命中某个在线串号
//  3. 整体包含某个在线串号
//  4. 只在线一台设备时直接用它
//
// 全部失败时返回错误并列出在线串号，便于一眼看出问题。
func resolveSerial(adbPath, id string) (string, error) {
	id = strings.TrimSpace(id)
	online := listADBSerials(adbPath)

	if len(online) == 0 {
		return "", fmt.Errorf("no adb device online")
	}
	for _, s := range online {
		if s == id {
			return s, nil
		}
	}
	for _, part := range strings.Split(id, "_") {
		if part == "" {
			continue
		}
		for _, s := range online {
			if part == s {
				return s, nil
			}
		}
	}
	if id != "" {
		for _, s := range online {
			if s != "" && strings.Contains(id, s) {
				return s, nil
			}
		}
	}
	if len(online) == 1 {
		return online[0], nil
	}
	return "", fmt.Errorf("cannot resolve device id %q to an adb serial; online: %s",
		id, strings.Join(online, ", "))
}

// listADBSerials 返回当前处于 device 状态（已授权、在线）的 adb 设备串号。
func listADBSerials(adbPath string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, adbPath, "devices").Output()
	if err != nil {
		return nil
	}
	var serials []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "device" {
			serials = append(serials, fields[0])
		}
	}
	return serials
}

// runDeviceShell 在受控设备上以 root 执行一段 shell：
//   - serial 非空 → 通过 adb（NAS 托管模式，webscreen 跑在 NAS 上）
//   - serial 为空 → 直接在本机执行（APK 内嵌模式，服务就跑在手机里）
func runDeviceShell(serial, script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 脚本用 base64 传输：`adb shell su -c "<script>"` 会把参数按空格重新拼接，
	// 多行脚本的换行与引号会被破坏（真机实测：命令返回 0 但背光没变），
	// 因此统一编码后由远端 shell 解码执行。
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	wrapped := fmt.Sprintf("echo %s | base64 -d | sh", encoded)

	// 本机模式（APK 内嵌：服务就跑在被控设备上）时必须直接在本机执行 ——
	// 此时设备没有 adb 也不需要 adb。注意不能只看 serial 是否为空：
	// 前端从 URL 拿到的是设备 id（例如 /screen/10fda345），本机模式下这个 id
	// 并不是可用的 adb 串号，走 adb 分支会失败（真机实测报错
	// "webscreen is running on the device itself (root mode), adb is neither
	// available nor required"）。exec 直接传参，不经 shell 拼接，无需引号。
	if strings.TrimSpace(serial) == "" || utils.IsLocalRootMode() {
		cmd := exec.CommandContext(ctx, "/system/bin/su", "-c", wrapped)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	adbPath, err := utils.GetADBPath()
	if err != nil {
		return "", err
	}
	// 页面传来的标识不一定是裸串号（可能是 android_<serial> 或 <serial>_0_0），
	// 用在线设备列表解析出真正的串号，否则 `adb -s <id>` 会报 device not found。
	serial, err = resolveSerial(adbPath, serial)
	if err != nil {
		return "", err
	}
	// 关键：`adb shell` 会把它收到的参数用空格拼成一条远程命令。
	// 若直接传 "su", "-c", wrapped，远程实际执行的是
	//   su -c echo <b64> | base64 -d | sh
	// —— 管道被外层 shell 截获，su 只拿到 "echo"，脚本在本地乱跑，
	// 远端背光不变而退出码仍为 0（真机实测：API 返回 success 但 brightness 没变）。
	// 因此必须把 "su -c '<script>'" 作为**单个参数**交给 adb。
	quoted := "'" + strings.ReplaceAll(wrapped, "'", `'\''`) + "'"
	cmd := exec.CommandContext(ctx, adbPath, "-s", serial, "shell", "su -c "+quoted)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (wm *WebMaster) handleBacklight(c *gin.Context) {
	var req backlightRequest
	if err := c.BindJSON(&req); err != nil || req.Off == nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": "error", "message": "invalid request"})
		return
	}

	script := backlightOnScript
	if *req.Off {
		script = backlightOffScript
	}

	out, err := runDeviceShell(req.Serial, script)
	trimmed := strings.TrimSpace(out)
	if err != nil || !strings.Contains(trimmed, "WEBSREEN_BACKLIGHT_OK") {
		msg := trimmed
		if err != nil {
			msg = trimmed + " " + err.Error()
		}
		log.Printf("[backlight] failed serial=%q off=%v: %s", req.Serial, *req.Off, msg)
		c.JSON(http.StatusInternalServerError, gin.H{"result": "error", "message": msg})
		return
	}
	log.Printf("[backlight] serial=%q off=%v ok", req.Serial, *req.Off)
	c.JSON(http.StatusOK, gin.H{"result": "success", "off": *req.Off})
}
