package scrcpy

import "bytes"

// PruneSEI 高效去除 SEI 和 AUD
// 采用原地复用底层数组的方式，避免多次内存搬运和重新分配内存
// 注意：该函数会改变切片长度，调用者必须接收返回值更新切片，例如：payload = PruneSEI(payload, codec)
func PruneSEI(payload []byte, codec string) []byte {
	// 如果包太小，直接返回
	if len(payload) < 5 {
		return payload
	}

	// 1. 复用底层数组：out 是结果切片，初始长度为 0，但容量与 payload 相同
	// 这样 append 操作不会触发内存分配，而是直接在原内存上重写
	out := payload[:0]

	// 查找第一个起始码的位置
	// 注意：FFmpeg 输出通常是 00 00 00 01，但也可能是 00 00 01
	// 这里沿用你代码中的 4 字节起始码设定
	start := 0

	// 处理开头的非 NALU 数据（如果有）
	firstStart := bytes.Index(payload, startCode)
	if firstStart == -1 {
		return payload // 没有起始码，直接返回
	}

	// 如果开头有一段垃圾数据，先保留（或者根据需求丢弃，这里保留以防误删）
	if firstStart > 0 {
		out = append(out, payload[:firstStart]...)
		start = firstStart
	}

	totalLen := len(payload)

	for start < totalLen {
		// 寻找下一个 NALU 的起始位置
		// 从当前 NALU 的 Data 部分开始找（跳过当前的 Header 4字节）
		// 避免由 00 00 00 01 自身造成的死循环
		var nextStartRelative int
		if start+4 < totalLen {
			nextStartRelative = bytes.Index(payload[start+4:], startCode)
		} else {
			nextStartRelative = -1
		}

		end := totalLen
		if nextStartRelative != -1 {
			// 下一个起始码的绝对位置
			end = start + 4 + nextStartRelative
		}

		// 此时 payload[start:end] 就是一个完整的 NALU (包含起始码)
		nalUnit := payload[start:end]

		// 检查是否需要保留
		if !isSEIOrAUD(nalUnit, codec) {
			// 需要保留：将数据追加到 out
			// 由于 out 和 payload 共享底层数组，且 out 的增长速度 <= start 的增长速度
			// 所以这里实际上是把后面的数据“搬”到前面，不会覆盖还未读取的数据
			out = append(out, nalUnit...)
		}

		// 移动游标
		start = end
	}

	return out
}

// isSEIOrAUD 判断 NALU 是否为 SEI 或 AUD
func isSEIOrAUD(nalWithHeader []byte, codec string) bool {
	// 确保长度足够读取 NAL Header (至少 4字节起始码 + 1字节 Header)
	if len(nalWithHeader) < 5 {
		return false
	}

	// 第 5 个字节是 NAL Header
	header := nalWithHeader[4]
	var nalType byte

	if codec == "h264" {
		// H.264: 低 5 位是 Type
		nalType = header & 0x1F
		// 6: SEI, 9: AUD
		return nalType == 6 || nalType == 9
	} else if codec == "h265" {
		// H.265: 中间 6 位是 Type (右移1位)
		nalType = (header >> 1) & 0x3F
		// 39: Prefix SEI, 40: Suffix SEI, 35: AUD
		return nalType == 39 || nalType == 40 || nalType == 35
	}

	return false
}
