package gamestream

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"time"
)

const (
	ctrlChannelCount   = 0x30
	ctrlChannelGeneric = 0x00
	ctrlChannelUrgent  = 0x01
	ctrlChannelKey     = 0x02
	ctrlChannelMouse   = 0x03
	ctrlChannelTouch   = 0x05

	ctrlTypePeriodicPing = 0x0200
	ctrlTypeInputData    = 0x0206
	ctrlTypeStartA       = 0x0305
	ctrlTypeRequestIDR   = 0x0302
	ctrlTypeStartB       = 0x0307
)

type controlStream struct {
	enet      *enetClient
	encrypted bool
	controlV2 bool
	aead      cipher.AEAD

	mu   sync.Mutex
	seq  uint32
	done chan struct{}
}

func startControlStream(ip string, port int, connectData uint32, rikey [16]byte, encrypted bool, controlV2 bool) (*controlStream, error) {
	remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(ip, itoa(port)))
	if err != nil {
		return nil, err
	}

	enet, err := dialENet(remoteAddr, ctrlChannelCount, connectData, 5*time.Second)
	if err != nil {
		return nil, err
	}

	var aead cipher.AEAD
	if encrypted {
		blk, err := aes.NewCipher(rikey[:])
		if err != nil {
			enet.close()
			return nil, err
		}
		if controlV2 {
			aead, err = cipher.NewGCM(blk)
		} else {
			aead, err = cipher.NewGCMWithNonceSize(blk, 16)
		}
		if err != nil {
			enet.close()
			return nil, err
		}
	}

	cs := &controlStream{
		enet:      enet,
		encrypted: encrypted,
		controlV2: controlV2,
		aead:      aead,
		done:      make(chan struct{}),
	}

	// Send Start A & Start B
	startAType := uint16(ctrlTypeStartA)
	if encrypted {
		startAType = ctrlTypeRequestIDR // Gen7Enc IDX_START_A == 0x0302
	}
	if err := cs.send(startAType, []byte{0, 0}, ctrlChannelGeneric, true); err != nil {
		cs.close()
		return nil, err
	}
	if err := cs.send(ctrlTypeStartB, []byte{0}, ctrlChannelGeneric, true); err != nil {
		cs.close()
		return nil, err
	}

	go cs.periodicPingLoop()
	return cs, nil
}

func (cs *controlStream) send(ptype uint16, payload []byte, channelID uint8, reliable bool) error {
	if cs == nil || cs.enet == nil {
		return errors.New("control stream not initialized")
	}

	var wire []byte
	if cs.encrypted {
		cs.mu.Lock()
		seq := cs.seq
		cs.seq++
		cs.mu.Unlock()

		wire = encryptControlPacket(cs.aead, seq, ptype, payload, cs.controlV2)
	} else {
		wire = make([]byte, 4+len(payload))
		binary.LittleEndian.PutUint16(wire[0:2], ptype)
		binary.LittleEndian.PutUint16(wire[2:4], uint16(len(payload)))
		copy(wire[4:], payload)
	}

	if reliable {
		return cs.enet.sendReliable(channelID, wire)
	}
	return cs.enet.sendUnsequenced(channelID, wire)
}

func encryptControlPacket(aead cipher.AEAD, seq uint32, ptype uint16, payload []byte, controlV2 bool) []byte {
	inner := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint16(inner[0:2], ptype)
	binary.LittleEndian.PutUint16(inner[2:4], uint16(len(payload)))
	copy(inner[4:], payload)

	var iv []byte
	if controlV2 {
		var iv12 [12]byte
		binary.LittleEndian.PutUint32(iv12[0:4], seq)
		iv12[10] = 'C'
		iv12[11] = 'C'
		iv = iv12[:]
	} else {
		var iv16 [16]byte
		iv16[0] = byte(seq)
		iv = iv16[:]
	}

	sealed := aead.Seal(nil, iv, inner, nil)
	ctLen := len(inner)
	ct := sealed[:ctLen]
	tag := sealed[ctLen:]

	out := make([]byte, 4+4+16+ctLen)
	binary.LittleEndian.PutUint16(out[0:2], 0x0001)
	binary.LittleEndian.PutUint16(out[2:4], uint16(4+16+ctLen))
	binary.LittleEndian.PutUint32(out[4:8], seq)
	copy(out[8:24], tag)
	copy(out[24:], ct)
	return out
}

func (cs *controlStream) requestIDR() error {
	return cs.send(ctrlTypeRequestIDR, []byte{0, 0}, ctrlChannelUrgent, true)
}

func (cs *controlStream) periodicPingLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var pingPayload [8]byte
	binary.LittleEndian.PutUint32(pingPayload[0:4], 4)
	binary.LittleEndian.PutUint32(pingPayload[4:8], 0)

	for {
		select {
		case <-cs.done:
			return
		case <-ticker.C:
			_ = cs.send(ctrlTypePeriodicPing, pingPayload[:], ctrlChannelGeneric, true)
		}
	}
}

func (cs *controlStream) close() {
	if cs == nil {
		return
	}
	select {
	case <-cs.done:
		return
	default:
		close(cs.done)
	}
	if cs.enet != nil {
		cs.enet.close()
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
