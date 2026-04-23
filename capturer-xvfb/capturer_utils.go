package main

import (
	"bytes"
	"log"
	"net"
	"os/exec"
	"strings"
)

func WaitTCP(port string) net.Conn {
	var err error
	var conn net.Conn
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Println("Failed to start video listener:", err)
	}
	conn, err = listener.Accept()
	if err != nil {
		log.Println("Failed to accept connection:", err)
	}
	listener.Close()
	log.Println("TCP connection established:", port)

	return conn
}

// GetBestH264Encoder 自动检测最佳 H.264 编码器
func GetBestH264Encoder() string {
	// 1. 检查是否存在瑞芯微硬件编码器 (Rockchip)
	if hasEncoder("h264_rkmpp") {
		return "h264_rkmpp"
	}

	// 2. 检查是否存在 NVIDIA 硬件编码器 (PC N卡)
	if hasEncoder("h264_nvenc") {
		return "h264_nvenc"
	}

	// 3. 检查是否存在 Intel 硬件编码器 (PC 核显)
	if hasEncoder("h264_qsv") {
		return "h264_qsv"
	}

	// 4. Android Termux 下常用的硬件编码 (MediaCodec)
	if hasEncoder("h264_mediacodec") {
		return "h264_mediacodec"
	}

	// 5. 默认回退到软件编码 (通用)
	return "libx264"
}

// GetBestHEVCEncoder 自动检测最佳 H.265 (HEVC) 编码器
func GetBestHEVCEncoder() string {
	// 1. Rockchip 瑞芯微
	if hasEncoder("hevc_rkmpp") {
		return "hevc_rkmpp"
	}

	// 2. NVIDIA (PC N卡)
	if hasEncoder("hevc_nvenc") {
		return "hevc_nvenc"
	}

	// 3. Intel QSV (PC 核显)
	if hasEncoder("hevc_qsv") {
		return "hevc_qsv"
	}

	// 4. AMD AMF (PC A卡)
	if hasEncoder("hevc_amf") {
		return "hevc_amf"
	}

	// 5. Apple VideoToolbox (Mac)
	if hasEncoder("hevc_videotoolbox") {
		return "hevc_videotoolbox"
	}

	// 6. Android/Termux MediaCodec (手机通用)
	if hasEncoder("hevc_mediacodec") {
		return "hevc_mediacodec"
	}

	// 7. VAAPI (Linux 通用硬件加速接口)
	if hasEncoder("hevc_vaapi") {
		return "hevc_vaapi"
	}

	// 8. 软件编码
	// 除非是强劲的 PC CPU，否则在 ARM 上跑 libx265 可能会只有 1-5 FPS
	return "libx265"
}

// hasEncoder 运行 ffmpeg -encoders 并检查输出
func hasEncoder(name string) bool {
	cmd := exec.Command("ffmpeg", "-encoders")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), name)
}

// splitNALU 是 bufio.SplitFunc 的实现，用于切分 H.264 Annex B 流
// 它会返回包含起始码（00 00 00 01）在内的完整 NALU
func splitNALU(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// 查找起始码前缀 00 00 01
	// H.264 起始码可能是 00 00 01 (3 bytes) 或 00 00 00 01 (4 bytes)
	// 我们主要查找 00 00 01，然后判断前面是否还有一个 0

	// start 变量记录当前包的起始位置（包含起始码）
	start := 0

	// 如果数据开头不是 00 00 01 或 00 00 00 01，说明可能有些垃圾数据或上一包的残余（理论上不应发生）
	// 但为了健壮性，我们可以先定位到第一个起始码
	firstStart := bytes.Index(data, []byte{0, 0, 1})
	if firstStart == -1 {
		if atEOF {
			return len(data), nil, nil // 丢弃无法解析的数据
		}
		return 0, nil, nil // 等待更多数据
	}

	// 调整 start 位置，包含前导的 00 (如果是 4 字节起始码)
	if firstStart > 0 && data[firstStart-1] == 0 {
		start = firstStart - 1
	} else {
		start = firstStart
	}

	// 从 start + 3 开始查找*下一个*起始码
	// +3 是为了跳过当前的 00 00 01
	nextStart := bytes.Index(data[start+3:], []byte{0, 0, 1})

	if nextStart != -1 {
		// 找到了下一个起始码的位置（相对于 data[start+3:]）
		// 绝对位置是 start + 3 + nextStart
		end := start + 3 + nextStart

		// 检查下一个起始码前面是否还有一个 0 (构成 00 00 00 01)
		if data[end-1] == 0 {
			end--
		}

		// 返回当前完整的 NALU (从 start 到 end)
		// advance 设为 end，表示我们消费了这么多数据
		return end, data[start:end], nil
	}

	// 如果没找到下一个起始码，且已经 EOF，则剩余所有数据就是一个 NALU
	if atEOF {
		return len(data), data[start:], nil
	}

	// 如果没找到下一个起始码，且未 EOF，则请求更多数据
	// 注意：这里如果不返回 start 之前的垃圾数据，会导致 buffer 堆积，
	// 所以如果 start > 0，我们应该先丢弃 start 之前的垃圾
	if start > 0 {
		return start, nil, nil
	}

	return 0, nil, nil
}

func getNalType(data []byte) byte {
	// 简单的辅助函数，用于日志，需跳过起始码
	// 找到第一个非0字节（通常是1），下一字节就是 header
	for i := 0; i < len(data)-1; i++ {
		if data[i] == 0 && data[i+1] == 1 {
			if i+2 < len(data) {
				return data[i+2] & 0x1F
			}
		}
	}
	return 0
}
