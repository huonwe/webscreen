package scrcpy

import (
	"bytes"
	"log"
	"time"
	"webscreen/sdriver"
)

var startCode []byte = []byte{0x00, 0x00, 0x00, 0x01}

func (da *ScrcpyDriver) updateCache(payload []byte, codec string) {
	// Scrcpy 始终使用 4 字节起始码

	// 游标：指向当前 NALU 数据的起始位置
	pos := 0

	// 如果包头就是起始码，直接跳过
	if bytes.HasPrefix(payload, startCode) {
		pos = 4
	}

	totalLen := len(payload)

	var nalType byte
	da.cacheMutex.Lock()
	defer da.cacheMutex.Unlock()
	for pos < totalLen {
		// 1. 查找下一个起始码的位置 (使用汇编优化的 bytes.Index)
		// 注意：搜索范围是 payload[pos:]，返回的是相对偏移量
		nextStartRelative := bytes.Index(payload[pos:], startCode)

		var end int
		if nextStartRelative == -1 {
			// 后面没有起始码了，说明当前 NALU 一直到包尾
			end = totalLen
		} else {
			// 当前 NALU 结束位置 = 当前起始位置 + 相对偏移量
			end = pos + nextStartRelative
		}

		// 2. 获取 Raw NALU (不含起始码，零拷贝切片)
		nal := payload[pos:end]

		// 更新游标到下一个 NALU 的数据开始处 (跳过 4 字节起始码)
		pos = end + 4

		if len(nal) == 0 {
			continue
		}

		switch codec {
		case "h265":
			nalType = (nal[0] >> 1) & 0x3F
		case "h264":
			nalType = nal[0] & 0x1F
		default:
			log.Println("Unknown codec type for NALU parsing:", codec)
			return
		}
		switch nalType {
		case 32: // VPS
			// log.Println("cached VPS")
			da.LastVPS = createCopy(nal)
		case 7, 33: // SPS
			// log.Println("cached SPS")
			da.updateVideoMetaFromSPS(nal, codec)
			da.LastSPS = createCopy(nal)
		case 8, 34: // PPS
			// log.Println("cached PPS")
			da.LastPPS = createCopy(nal)
		case 5, 19, 20, 21: // IDR
			// log.Println("cached IDR")
			da.LastIDR = createCopy(nal)
		default:
			// 其他类型暂不处理
		}
	}
}

func (da *ScrcpyDriver) sendCachedKeyFrame() {
	da.cacheMutex.RLock()
	cachedVPS := createCopy(da.LastVPS)
	cachedSPS := createCopy(da.LastSPS)
	cachedPPS := createCopy(da.LastPPS)
	cachedIDR := createCopy(da.LastIDR)
	lastPTS := da.LastPTS
	da.cacheMutex.RUnlock()

	var merged_data []byte
	if len(cachedVPS) > 0 {
		merged_data = append(merged_data, startCode...)
		merged_data = append(merged_data, cachedVPS...)
	}
	merged_data = append(merged_data, startCode...)
	merged_data = append(merged_data, cachedSPS...)
	merged_data = append(merged_data, startCode...)
	merged_data = append(merged_data, cachedPPS...)
	merged_data = append(merged_data, startCode...)
	merged_data = append(merged_data, cachedIDR...)
	log.Println("⚡ Sending cached key frame and parameter sets")
	da.VideoChan <- sdriver.AVBox{Data: merged_data, PTS: lastPTS, IsKeyFrame: true, IsConfig: false}
}

func (da *ScrcpyDriver) sendWithCachedConfigFrame(PTS time.Duration, IDRFrame []byte) {
	da.cacheMutex.RLock()
	cachedVPS := createCopy(da.LastVPS)
	cachedSPS := createCopy(da.LastSPS)
	cachedPPS := createCopy(da.LastPPS)
	da.cacheMutex.RUnlock()

	var merged_data []byte
	if len(cachedVPS) > 0 {
		merged_data = append(merged_data, startCode...)
		merged_data = append(merged_data, cachedVPS...)
	}
	merged_data = append(merged_data, startCode...)
	merged_data = append(merged_data, cachedSPS...)
	merged_data = append(merged_data, startCode...)
	merged_data = append(merged_data, cachedPPS...)
	merged_data = append(merged_data, IDRFrame...)
	log.Println("⚡ wrap with cached parameter sets")
	// parts := bytes.Split(merged_data, startCode)
	// for i, part := range parts {
	// 	if len(part) == 0 {
	// 		continue
	// 	}
	// 	log.Printf("  Part %d: NALU Type=%d, Size=%d bytes\n", i, (part[0]>>1)&0x3F, len(part))
	// }

	da.VideoChan <- sdriver.AVBox{Data: merged_data, PTS: PTS, IsKeyFrame: true, IsConfig: false}
}
