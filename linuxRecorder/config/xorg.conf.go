package config

import (
	"fmt"
	"strings"
)

func GetXorgConfig(driver string, width int, height int, depth int) string {
	var builder strings.Builder
	builder.WriteString("Section \"ServerFlags\"\n")
	builder.WriteString("    Option \"AutoAddGPU\" \"false\"\n")
	builder.WriteString("    Option \"AutoBindGPU\" \"false\"\n")
	builder.WriteString("EndSection\n\n")

	builder.WriteString("Section \"Monitor\"\n")
	builder.WriteString("    Identifier \"Monitor0\"\n")
	builder.WriteString("    Option \"DPMS\" \"false\"\n")
	builder.WriteString("EndSection\n\n")

	builder.WriteString("Section \"Device\"\n")
	builder.WriteString("    Identifier \"Device0\"\n")
	switch driver {
	case "nvidia":
		builder.WriteString("    Driver \"nvidia\"\n")
		builder.WriteString("    Option \"AllowEmptyInitialConfiguration\" \"True\"\n")
		builder.WriteString("    Option \"UseDisplayDevice\" \"None\"\n")
	case "modesetting":
		builder.WriteString("    Driver \"modesetting\"\n")
		// 你的代码里写了 DRI 3，由于之前我们分析过 Vendor 驱动更稳的是 DRI 2，建议改回 2 试试
		builder.WriteString("    Option \"DRI\" \"3\"\n")
		builder.WriteString("    Option \"AccelMethod\" \"glamor\"\n")
		builder.WriteString("    Option \"kmsdev\" \"/dev/dri/card0\"\n")

		// 👇 核心修改 1：使用 SWcursor 明确强制软件渲染
		builder.WriteString("    Option \"SWcursor\" \"on\"\n")
		// 注释掉 HWCursor，防止参数解析冲突
		// builder.WriteString("    Option \"HWCursor\" \"off\"\n")

		builder.WriteString("    Option \"AllowEmptyInitialConfiguration\" \"True\"\n")

		// 👇 核心修改 2：千万不要开 ShadowPrimary，它会导致软件鼠标无法输出到 DRM Plane
		// builder.WriteString("    Option \"ShadowPrimary\" \"true\"\n")
	case "dummy":
		builder.WriteString("    Driver \"dummy\"\n")
		builder.WriteString("    VideoRam 256000\n")
	default:
		builder.WriteString("    Driver \"dummy\"\n")
		builder.WriteString("    VideoRam 256000\n")
	}
	builder.WriteString("EndSection\n\n")

	builder.WriteString("Section \"Screen\"\n")
	builder.WriteString("    Identifier \"Screen0\"\n")
	builder.WriteString("    Device \"Device0\"\n")
	builder.WriteString("    Monitor \"Monitor0\"\n")
	builder.WriteString("    DefaultDepth ")
	builder.WriteString(fmt.Sprintf("%d\n", depth))
	builder.WriteString("    SubSection \"Display\"\n")
	builder.WriteString(fmt.Sprintf("        Depth %d\n", depth))
	builder.WriteString(fmt.Sprintf("        Modes \"%dx%d\"\n", width, height))
	builder.WriteString(fmt.Sprintf("        Virtual %d %d\n", width, height))
	builder.WriteString("    EndSubSection\n")
	builder.WriteString("EndSection\n\n")
	return builder.String()
}
