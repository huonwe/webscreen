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
	var curFrameCorrupt bool
	var lastEmittedFrameIdx uint32
	var hasEmittedFrame bool
	var completedBlockFrame uint32
	var completedBlockNum int
	var hasCompletedBlock bool
	waitingForIDR := true

	resetFrame := func() {
		for b := 0; b < len(frameBlocks); b++ {
			frameBlocks[b] = nil
		}
		curFrameInit = false
		curFrameCorrupt = false
	}

	flushFrame := func(frameIdx uint32, expectedLastBlock int) {
		if curFrameCorrupt {
			resetFrame()
			return
		}
		var frameBuf []byte
		for b := 0; b <= expectedLastBlock && b < len(frameBlocks); b++ {
			if frameBlocks[b] == nil {
				// Missing a block in this frame; drop the whole frame and wait for IDR.
				resetFrame()
				waitingForIDR = true
				d.RequestIDR(false)
				return
			}
			for _, p := range frameBlocks[b] {
				frameBuf = append(frameBuf, p...)
			}
		}
		resetFrame()
		if len(frameBuf) == 0 {
			return
		}
		if hasEmittedFrame && int32(frameIdx-lastEmittedFrameIdx) <= 0 {
			return
		}
		lastEmittedFrameIdx = frameIdx
		hasEmittedFrame = true
		d.emitVideoFrame(frameBuf, frameIdx, &waitingForIDR)
	}

	completeBlock := func(blk *fecBlock) {
		if blk == nil || blk.dataShards <= 0 {
			return
		}
		if curFrameInit && blk.frameIndex != curFrameIdx {
			// Previous frame never saw its final block; discard it cleanly.
			resetFrame()
			waitingForIDR = true
			d.RequestIDR(false)
		}
		curFrameIdx = blk.frameIndex
		curFrameInit = true

		if blk.receivedData < blk.dataShards {
			if blk.receivedCount >= blk.dataShards && blk.parityShards > 0 {
				shardLen := rtsp.videoPacketSize + rtpHeaderSize + 4
				for i := range blk.shards {
					if blk.present[i] && len(blk.shards[i]) > shardLen {
						shardLen = len(blk.shards[i])
					}
				}
				for i := range blk.shards {
					if blk.present[i] && len(blk.shards[i]) < shardLen {
						padded := make([]byte, shardLen)
						copy(padded, blk.shards[i])
						blk.shards[i] = padded
					}
				}
				if err := rsDecode(blk.shards, blk.present, blk.dataShards, blk.parityShards, shardLen); err != nil {
					log.Printf("[gamestream] video frame %d block %d FEC failed (%v); requesting IDR", blk.frameIndex, blk.blockNumber, err)
					curFrameCorrupt = true
					waitingForIDR = true
					d.RequestIDR(false)
					return
				}
			} else {
				curFrameCorrupt = true
				waitingForIDR = true
				d.RequestIDR(false)
				return
			}
		}

		// Extract payloads from data shards
		payloads := make([][]byte, 0, blk.dataShards)
		for i := 0; i < blk.dataShards; i++ {
			pkt := blk.shards[i]
			if !blk.present[i] || len(pkt) == 0 {
				continue
			}
			hdrLen := rtpHeaderSize
			if pkt[0]&rtpFlagExtension != 0 {
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

		if blk.blockNumber >= 0 && blk.blockNumber < len(frameBlocks) {
			frameBlocks[blk.blockNumber] = payloads
		}
		if blk.blockNumber >= blk.lastBlock {
			flushFrame(blk.frameIndex, blk.lastBlock)
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

		if hasCompletedBlock && frameIndex == completedBlockFrame && blockNum == completedBlockNum {
			// Trailing FEC parity or duplicate packets for an already-completed block; ignore.
			continue
		}
		if hasEmittedFrame && int32(frameIndex-lastEmittedFrameIdx) <= 0 {
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
			completedBlockFrame = curBlock.frameIndex
			completedBlockNum = curBlock.blockNumber
			hasCompletedBlock = true
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

var annexBStartCode = []byte{0x00, 0x00, 0x00, 0x01}

func (d *Driver) emitVideoFrame(annexB []byte, frameIdx uint32, waitingForIDR *bool) bool {
	ptsUs := uint64(time.Since(d.startTime).Microseconds())
	isIDR := false

	// Split Annex-B stream into NALs (stripping AUD/SEI), keeping track of config NALs and slices.
	var vclNALs [][]byte
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
		if len(nal) > 0 {
			if d.updateConfigOrCheckIDR(nal, &isIDR) {
				vclNALs = append(vclNALs, nal)
			}
		}
		if offset >= len(annexB) {
			break
		}
	}

	if *waitingForIDR {
		if !isIDR {
			d.RequestIDR(false)
			return false
		}
		*waitingForIDR = false
	}

	if len(vclNALs) == 0 {
		return isIDR
	}

	var out []byte
	if isIDR {
		// Bundle VPS (if H.265), SPS, PPS, and IDR slices together with Annex-B start codes
		// so WebCodecs and WebRTC decoders always receive a self-contained keyframe.
		d.mu.RLock()
		for _, cfg := range [][]byte{d.lastVPS, d.lastSPS, d.lastPPS} {
			if len(cfg) > 0 {
				out = append(out, annexBStartCode...)
				out = append(out, cfg...)
			}
		}
		d.mu.RUnlock()
	}
	for _, nal := range vclNALs {
		out = append(out, annexBStartCode...)
		out = append(out, nal...)
	}

	if isIDR {
		d.mu.Lock()
		d.lastIDR = append([]byte(nil), out...)
		d.mu.Unlock()
	}

	box := sdriver.AVBox{
		Data: out,
		PTS:  ptsUs,
		Type: 0,
	}
	select {
	case d.videoChan <- box:
	case <-d.done:
	}
	return isIDR
}

// updateConfigOrCheckIDR caches VPS/SPS/PPS (without start codes), drops AUD/SEI,
// and returns true if nal is a VCL slice to emit.
func (d *Driver) updateConfigOrCheckIDR(nal []byte, isIDR *bool) bool {
	if d.meta.VideoCodec == "h265" {
		if len(nal) < 2 {
			return false
		}
		nalType := (nal[0] >> 1) & 0x3f
		if nalType == 35 || nalType == 39 || nalType == 40 {
			return false
		}
		d.mu.Lock()
		switch nalType {
		case 32:
			d.lastVPS = append([]byte(nil), nal...)
			d.mu.Unlock()
			return false
		case 33:
			d.lastSPS = append([]byte(nil), nal...)
			d.mu.Unlock()
			return false
		case 34:
			d.lastPPS = append([]byte(nil), nal...)
			d.mu.Unlock()
			return false
		}
		d.mu.Unlock()
		if nalType >= 19 && nalType <= 21 {
			*isIDR = true
		}
		return true
	}

	nalType := nal[0] & 0x1f
	if nalType == 9 || nalType == 6 {
		return false
	}
	d.mu.Lock()
	switch nalType {
	case 7:
		d.lastSPS = append([]byte(nil), nal...)
		d.mu.Unlock()
		return false
	case 8:
		d.lastPPS = append([]byte(nil), nal...)
		d.mu.Unlock()
		return false
	}
	d.mu.Unlock()
	if nalType == 5 {
		*isIDR = true
	}
	return true
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
