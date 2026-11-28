package gamestream

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"net"
	"scrmux/sdriver"
	"time"
)

const (
	rtpPayloadAudioOpus = 97
	rtpPayloadAudioFEC  = 127
)

func (d *Driver) audioLoop(conn *net.UDPConn, remoteAddr *net.UDPAddr, rtsp *rtspResult, rikey [16]byte, rikeyID uint32) {
	defer close(d.audioChan)

	var block cipher.Block
	if rtsp.audioEncrypted {
		block, _ = aes.NewCipher(rikey[:])
	}

	go d.udpPingLoop(conn, remoteAddr, rtsp.audioPingPayload, rtsp.hasAudioPing)

	buf := make([]byte, 2048)
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
		if len(pkt) < rtpHeaderSize {
			continue
		}

		pt := pkt[1] & 0x7F
		if pt == rtpPayloadAudioFEC || pt != rtpPayloadAudioOpus {
			continue
		}

		seq := binary.BigEndian.Uint16(pkt[2:4])
		payload := pkt[rtpHeaderSize:]
		if len(payload) == 0 {
			continue
		}

		if block != nil {
			if len(payload)%aes.BlockSize != 0 {
				continue
			}
			var iv [16]byte
			binary.BigEndian.PutUint32(iv[:4], rikeyID+uint32(seq))
			decrypted := make([]byte, len(payload))
			cipher.NewCBCDecrypter(block, iv[:]).CryptBlocks(decrypted, payload)
			unpadded, ok := pkcs7Unpad(decrypted, aes.BlockSize)
			if !ok || len(unpadded) == 0 {
				continue
			}
			payload = unpadded
		} else {
			cp := make([]byte, len(payload))
			copy(cp, payload)
			payload = cp
		}

		box := sdriver.AVBox{
			Data: payload,
			PTS:  uint64(time.Since(d.startTime).Microseconds()),
			Type: 1,
		}
		select {
		case d.audioChan <- box:
		case <-d.done:
			return
		}
	}
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, bool) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, false
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > blockSize || pad > len(data) {
		return nil, false
	}
	for i := len(data) - pad; i < len(data); i++ {
		if int(data[i]) != pad {
			return nil, false
		}
	}
	return data[:len(data)-pad], true
}
