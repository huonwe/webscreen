package config

import "fmt"

const swayHeadlessConf = `
# 基础配置
xwayland enable
output HEADLESS-1 resolution %dx%d@%sHz position 0 0 scale %s

# === 外观与美化配置 ===

# 1. 设置背景壁纸 (fill 模式会按比例缩放并裁剪填满屏幕)
output HEADLESS-1 bg /home/hiroi/Downloads/124956717_p0.png fill

# 2. 全局字体设置
font pango:sans-serif 11

# 3. 窗口边框与间距 (现代平铺桌面风格)
# 取消默认的粗大标题栏，改为 2 像素的纯色边框
default_border pixel 2
default_floating_border normal

# 设置窗口之间的缝隙，让壁纸能透出来
gaps inner 8
gaps outer 4

# 4. 窗口颜色配置 (基于优雅的 Nord 主题配色)
# 格式：class                 border  backgr. text    indicator child_border
client.focused          #88c0d0 #434c5e #eceff4 #8fbcbb   #88c0d0
client.focused_inactive #3b4252 #2e3440 #d8dee9 #4c566a   #4c566a
client.unfocused        #2e3440 #2e3440 #d8dee9 #2e3440   #2e3440
client.urgent           #bf616a #bf616a #eceff4 #bf616a   #bf616a

# 5. 状态栏配置 (可选)
# 如果你只想要一个纯净的画面（比如为了无干扰地跑特定应用），可以取消下面 bar 的注释来隐藏默认的底部状态栏
# bar {
#     mode invisible
# }

# 6. 设置 XWayland DPI (矢量清晰放大)
exec echo "Xft.dpi: %d" | xrdb -merge
`

func GetSwayHeadlessConf(width int, height int, frameRate string) string {
	// 默认 scale 1.0 (避免拉伸模糊), dpi 192 (200% 矢量放大)
	return GetSwayHeadlessConfWithScale(width, height, frameRate, "2.0", 192)
}

// GetSwayHeadlessConfWithScale 返回 Sway 无头配置，可以设置 DPI 缩放
// scale: 缩放因子，例如 "1.0"(100%)
// dpi: X11 缩放 DPI (96 为 100%, 192 为 200%)
func GetSwayHeadlessConfWithScale(width int, height int, frameRate string, scale string, dpi int) string {
	return fmt.Sprintf(swayHeadlessConf, width, height, frameRate, scale, dpi)
}
