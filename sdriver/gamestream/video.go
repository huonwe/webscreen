package gamestream

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"log"
	"net"
	"time"
	"webscreen/sdriver"
)

const (
	rtpHeaderSize      = 12
	nvVideoPacketSize  = 16
	rtpFlagExtension   = 0x10
	nvFlagContainsPic  = 0x01
	nvFlagEOF          = 0x02
	nvFlagSOF          = 0x04
	encVideoHeaderSize = 32 // 12B iv + 4B frameNumber + 16B tag
)

type fecBlock struct {
	frameIndex     uint32
	blockNumber    int
	lastBlock      int
	lowestSeq      uint16
	dataShards     int
	parityShards   int
	shards         [][]byte
	rawLengths     []int
	present        []bool
	receivedCount  int
	receivedData   int
	firstHeader    byte
	firstTimestamp uint32
	firstSSRC      uint32
	hasTemplate    bool
}

func (d *Driver) videoLoop(conn *net.UDPConn, remoteAddr *net.UDPAddr, rtsp *rtspResult, rikey [16]byte) {
	defer close(d.videoChan)

	var aead cipher.AEAD
	if rtsp.encryptionEnabled&ssEncVideo != 0 {
		blk, err := aes.NewCipher(rikey[:])
		if err == nil {
			aead, _ = cipher.NewGCM(blk)
		}
	}

	// Send UDP ping every 500ms to keep NAT / stream open.
	go d.udpPingLoop(conn, remoteAddr, rtsp.videoPingPayload, rtsp.hasVideoPing)

	buf := make([]byte, 4096)
	var curBlock *fecBlock
	var frameBlocks [4][][]byte // up to 4 FEC blocks per frame
	var curFrameIdx uint32
	var curFrameInit bool
	waitingForIDR := true

	flushFrame := func(frameIdx uint32) {
		var frameBuf []byte
		for b := 0; b < len(frameBlocks); b++ {
			for _, p := range frameBlocks[b] {
				frameBuf = append(frameBuf, p...)
			}
			frameBlocks[b] = nil
		}
		if len(frameBuf) == 0 {
			return
		}
		isIDR := d.emitVideoFrame(frameBuf, frameIdx)
		if isIDR {
			waitingForIDR = false
		} else if waitingForIDR {
			d.RequestIDR(false)
		}
	}

	completeBlock := func(blk *fecBlock) {
		if blk == nil || blk.dataShards <= 0 {
			return
		}
		if blk.receivedData < blk.dataShards {
			if blk.receivedCount >= blk.dataShards && blk.parityShards > 0 {
				shardLen := rtsp.videoPacketSize + rtpHeaderSize + 4
				for i := range blk.shards {
					if blk.present[i] && len(blk.shards[i]) < shardLen {
						padded := make([]byte, shardLen)
						copy(padded, blk.shards[i])
						blk.shards[i] = padded
					}
				}
				if err := rsDecode(blk.shards, blk.present, blk.dataShards, blk.parityShards, shardLen); err != nil {
					log.Printf("[gamestream] video frame %d block %d FEC failed (%v); requesting IDR", blk.frameIndex, blk.blockNumber, err)
					waitingForIDR = true
					d.RequestIDR(false)
					return
				}
			} else {
				waitingForIDR = true
				d.RequestIDR(false)
				return
			}
		}

		// Extract payloads from data shards
		payloads := make([][]byte, 0, blk.dataShards)
		for i := 0; i < blk.dataShards; i++ {
			pkt := blk.shards[i]
			hdrLen := rtpHeaderSize
			hdrByte := pkt[0]
			if !blk.present[i] {
				continue
			}
			if hdrByte&rtpFlagExtension != 0 {
				hdrLen += 4
			}
			if len(pkt) < hdrLen+nvVideoPacketSize {
				continue
			}
			payloadStart := hdrLen + nvVideoPacketSize
			end := len(pkt)
			if blk.rawLengths[i] > 0 && blk.rawLengths[i] < end {
				end = blk.rawLengths[i]
			}
			if payloadStart >= end {
				continue
			}
			payload := pkt[payloadStart:end]
			if i == 0 && blk.blockNumber == 0 {
				hdrSize := frameHeaderSize(payload)
				if hdrSize >= len(payload) {
					continue
				}
				payload = payload[hdrSize:]
			}
			payloads = append(payloads, payload)
		}

		if curFrameInit && blk.frameIndex != curFrameIdx {
			flushFrame(curFrameIdx)
		}
		curFrameIdx = blk.frameIndex
		curFrameInit = true
		if blk.blockNumber >= 0 && blk.blockNumber < len(frameBlocks) {
			frameBlocks[blk.blockNumber] = payloads
		}
		if blk.blockNumber >= blk.lastBlock {
			flushFrame(blk.frameIndex)
			curFrameInit = false
		}
	}

	for {
		select {
		case <-d.done:
			return
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return
		}
		pkt := buf[:n]

		if aead != nil {
			if len(pkt) <= encVideoHeaderSize {
				continue
			}
			iv := pkt[:12]
			tag := pkt[16:32]
			ct := pkt[32:]
			sealed := make([]byte, len(ct)+len(tag))
			copy(sealed, ct)
			copy(sealed[len(ct):], tag)
			plain, err := aead.Open(nil, iv, sealed, nil)
			if err != nil {
				continue
			}
			pkt = plain
		}

		if len(pkt) < rtpHeaderSize+nvVideoPacketSize {
			continue
		}
		rtpSeq := binary.BigEndian.Uint16(pkt[2:4])
		hdrLen := rtpHeaderSize
		if pkt[0]&rtpFlagExtension != 0 {
			hdrLen += 4
		}
		if len(pkt) < hdrLen+nvVideoPacketSize {
			continue
		}

		nv := pkt[hdrLen : hdrLen+nvVideoPacketSize]
		frameIndex := binary.LittleEndian.Uint32(nv[4:8])
		multiFecBlocks := nv[11]
		fecInfo := binary.LittleEndian.Uint32(nv[12:16])

		fecIndex := int((fecInfo & 0x3FF000) >> 12)
		dataShards := int((fecInfo & 0xFFC00000) >> 22)
		fecPct := int((fecInfo & 0xFF0) >> 4)
		blockNum := int((multiFecBlocks >> 4) & 0x3)
		lastBlock := int((multiFecBlocks >> 6) & 0x3)

		if dataShards <= 0 || dataShards > 255 {
			continue
		}
		parityShards := (dataShards*fecPct + 99) / 100
		totalShards := dataShards + parityShards
		if fecIndex < 0 || fecIndex >= totalShards {
			continue
		}

		if curBlock == nil || curBlock.frameIndex != frameIndex || curBlock.blockNumber != blockNum {
			if curBlock != nil {
				completeBlock(curBlock)
			}
			curBlock = &fecBlock{
				frameIndex:   frameIndex,
				blockNumber:  blockNum,
				lastBlock:    lastBlock,
				lowestSeq:    rtpSeq - uint16(fecIndex),
				dataShards:   dataShards,
				parityShards: parityShards,
				shards:       make([][]byte, totalShards),
				rawLengths:   make([]int, totalShards),
				present:      make([]bool, totalShards),
			}
		}

		if !curBlock.present[fecIndex] {
			cp := make([]byte, len(pkt))
			copy(cp, pkt)
			curBlock.shards[fecIndex] = cp
			curBlock.rawLengths[fecIndex] = len(pkt)
			curBlock.present[fecIndex] = true
			curBlock.receivedCount++
			if fecIndex < curBlock.dataShards {
				curBlock.receivedData++
			}
			if !curBlock.hasTemplate {
				curBlock.firstHeader = pkt[0]
				curBlock.firstTimestamp = binary.BigEndian.Uint32(pkt[4:8])
				curBlock.firstSSRC = binary.BigEndian.Uint32(pkt[8:12])
				curBlock.hasTemplate = true
			}
		}

		if curBlock.receivedData == curBlock.dataShards {
			completeBlock(curBlock)
			curBlock = nil
		}
	}
}

func frameHeaderSize(payload []byte) int {
	if len(payload) == 0 {
		return 0
	}
	if payload[0] == 0x81 {
		return 44
	}
	return 8
}

func (d *Driver) emitVideoFrame(annexB []byte, frameIdx uint32) bool {
	ptsUs := uint64(time.Since(d.startTime).Microseconds())
	isIDR := false

	offset := 0
	for {
		idx, startLen := findStartCode(annexB[offset:])
		if idx < 0 {
			break
		}
		nalStart := offset + idx + startLen
		nextIdx, _ := findStartCode(annexB[nalStart:])
		var nal []byte
		if nextIdx < 0 {
			nal = annexB[nalStart:]
			offset = len(annexB)
		} else {
			nal = annexB[nalStart : nalStart+nextIdx]
			offset = nalStart + nextIdx
		}
		nal = trimTrailingZeros(nal)
		if len(nal) == 0 {
			continue
		}
		if d.emitNAL(nal, ptsUs) {
			isIDR = true
		}
		if offset >= len(annexB) {
			break
		}
	}
	return isIDR
}

func trimTrailingZeros(b []byte) []byte {
	end := len(b)
	for end > 0 && b[end-1] == 0 {
		end--
	}
	return b[:end]
}

func (d *Driver) emitNAL(nal []byte, ptsUs uint64) bool {
	var nalType byte
	if d.meta.VideoCodec == "h265" {
		if len(nal) < 2 {
			return false
		}
		nalType = (nal[0] >> 1) & 0x3f
		// Drop HEVC AUD (35) and SEI (39, 40)
		if nalType == 35 || nalType == 39 || nalType == 40 {
			return false
		}
	} else {
		nalType = nal[0] & 0x1f
		// Drop H.264 AUD (9) and SEI (6)
		if nalType == 9 || nalType == 6 {
			return false
		}
	}

	isIDR := (d.meta.VideoCodec == "h264" && nalType == 5) ||
		(d.meta.VideoCodec == "h265" && nalType >= 19 && nalType <= 21)

	d.mu.Lock()
	switch {
	case d.meta.VideoCodec == "h265" && nalType == 32:
		d.lastVPS = append([]byte(nil), nal...)
	case d.meta.VideoCodec == "h265" && nalType == 33:
		d.lastSPS = append([]byte(nil), nal...)
	case d.meta.VideoCodec == "h265" && nalType == 34:
		d.lastPPS = append([]byte(nil), nal...)
	case d.meta.VideoCodec == "h264" && nalType == 7:
		d.lastSPS = append([]byte(nil), nal...)
	case d.meta.VideoCodec == "h264" && nalType == 8:
		d.lastPPS = append([]byte(nil), nal...)
	case isIDR:
		d.lastIDR = append([]byte(nil), nal...)
	}
	d.mu.Unlock()

	box := sdriver.AVBox{
		Data:       append([]byte(nil), nal...),
		PTS:        ptsUs,
		Type:       0,
		NoDuration: isConfigNAL(d.meta.VideoCodec, nalType),
	}
	select {
	case d.videoChan <- box:
	case <-d.done:
	}
	return isIDR
}

func isConfigNAL(codec string, nalType byte) bool {
	return (codec == "h264" && (nalType == 7 || nalType == 8)) ||
		(codec == "h265" && nalType >= 32 && nalType <= 34)
}

func findStartCode(data []byte) (int, int) {
	for i := 0; i+3 <= len(data); i++ {
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 1 {
			return i, 3
		}
		if i+4 <= len(data) && data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 1 {
			return i, 4
		}
	}
	return -1, 0
}

func (d *Driver) udpPingLoop(conn *net.UDPConn, remoteAddr *net.UDPAddr, payload [16]byte, hasPayload bool) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var seq uint32
	sendPing := func() {
		seq++
		if hasPayload {
			var msg [20]byte
			copy(msg[:16], payload[:])
			binary.BigEndian.PutUint32(msg[16:20], seq)
			_, _ = conn.WriteToUDP(msg[:], remoteAddr)
		} else {
			_, _ = conn.WriteToUDP([]byte("PING"), remoteAddr)
		}
	}

	sendPing()
	for {
		select {
		case <-d.done:
			return
		case <-ticker.C:
			sendPing()
		}
	}
}
