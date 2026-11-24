package gamestream

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	enetCmdAcknowledge     = 1
	enetCmdConnect         = 2
	enetCmdVerifyConnect   = 3
	enetCmdDisconnect      = 4
	enetCmdPing            = 5
	enetCmdSendReliable    = 6
	enetCmdSendUnsequenced = 8

	enetCmdFlagAck = 0x80
	enetCmdMask    = 0x0F

	enetHdrFlagSentTime  = 0x8000
	enetHdrSessionShift  = 12
	enetHdrPeerMask      = 0x0FFF
	enetDefaultPeerID    = 0x0FFF
	enetMaxWindowSize    = 32768
	enetDefaultMTU       = 1400
	enetProtocolThrottle = 5000
)

type enetPendingReliable struct {
	channelID uint8
	seq       uint16
	packet    []byte
	sentAt    time.Time
}

type enetClient struct {
	conn         *net.UDPConn
	remoteAddr   *net.UDPAddr
	channelCount uint32

	mu               sync.Mutex
	outgoingPeerID   uint16
	incomingSession  uint8
	outgoingSession  uint8
	outgoingRelSeq   []uint16
	outgoingUnseqGrp uint16
	pendingReliable  map[uint32]*enetPendingReliable
	connected        bool
	closed           bool

	connectAckCh chan error
	done         chan struct{}
}

func dialENet(remoteAddr *net.UDPAddr, channelCount uint32, connectData uint32, timeout time.Duration) (*enetClient, error) {
	conn, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		return nil, err
	}

	var randBytes [4]byte
	_, _ = rand.Read(randBytes[:])

	c := &enetClient{
		conn:            conn,
		remoteAddr:      remoteAddr,
		channelCount:    channelCount,
		outgoingPeerID:  enetDefaultPeerID,
		outgoingRelSeq:  make([]uint16, channelCount),
		pendingReliable: make(map[uint32]*enetPendingReliable),
		connectAckCh:    make(chan error, 1),
		done:            make(chan struct{}),
	}

	go c.recvLoop()
	go c.retransmitLoop()

	if err := c.sendConnect(connectData, binary.BigEndian.Uint32(randBytes[:])); err != nil {
		c.close()
		return nil, err
	}

	select {
	case err := <-c.connectAckCh:
		if err != nil {
			c.close()
			return nil, err
		}
		return c, nil
	case <-time.After(timeout):
		c.close()
		return nil, errors.New("enet: connect timeout")
	}
}

func (c *enetClient) sendConnect(connectData uint32, sessionID uint32) error {
	// ENetProtocolConnect (48 bytes) + ENetProtocolHeader (4 bytes with sentTime)
	pkt := make([]byte, 4+48)
	sentTime := uint16(time.Now().UnixMilli() & 0xFFFF)
	binary.BigEndian.PutUint16(pkt[0:2], enetDefaultPeerID|enetHdrFlagSentTime)
	binary.BigEndian.PutUint16(pkt[2:4], sentTime)

	cmd := pkt[4:]
	cmd[0] = enetCmdConnect | enetCmdFlagAck
	cmd[1] = 0xFF                                                // channelID
	binary.BigEndian.PutUint16(cmd[2:4], 1)                      // reliableSequenceNumber
	binary.BigEndian.PutUint16(cmd[4:6], 0)                      // outgoingPeerID (local peer 0)
	cmd[6] = c.incomingSession                                   // incomingSessionID
	cmd[7] = c.outgoingSession                                   // outgoingSessionID
	binary.BigEndian.PutUint32(cmd[8:12], enetDefaultMTU)        // mtu
	binary.BigEndian.PutUint32(cmd[12:16], enetMaxWindowSize)    // windowSize
	binary.BigEndian.PutUint32(cmd[16:20], c.channelCount)       // channelCount
	binary.BigEndian.PutUint32(cmd[20:24], 0)                    // incomingBandwidth
	binary.BigEndian.PutUint32(cmd[24:28], 0)                    // outgoingBandwidth
	binary.BigEndian.PutUint32(cmd[28:32], enetProtocolThrottle) // packetThrottleInterval
	binary.BigEndian.PutUint32(cmd[32:36], 2)                    // packetThrottleAcceleration
	binary.BigEndian.PutUint32(cmd[36:40], 2)                    // packetThrottleDeceleration
	binary.BigEndian.PutUint32(cmd[40:44], sessionID)            // connectID
	binary.BigEndian.PutUint32(cmd[44:48], connectData)          // data

	c.mu.Lock()
	c.outgoingRelSeq[0] = 1
	c.mu.Unlock()

	_, err := c.conn.Write(pkt)
	return err
}

func (c *enetClient) buildHeader(includeSentTime bool) []byte {
	peerField := (c.outgoingPeerID & enetHdrPeerMask) | (uint16(c.outgoingSession&0x3) << enetHdrSessionShift)
	if includeSentTime {
		peerField |= enetHdrFlagSentTime
		hdr := make([]byte, 4)
		binary.BigEndian.PutUint16(hdr[0:2], peerField)
		binary.BigEndian.PutUint16(hdr[2:4], uint16(time.Now().UnixMilli()&0xFFFF))
		return hdr
	}
	hdr := make([]byte, 2)
	binary.BigEndian.PutUint16(hdr[0:2], peerField)
	return hdr
}

func (c *enetClient) sendReliable(channelID uint8, payload []byte) error {
	c.mu.Lock()
	if c.closed || !c.connected {
		c.mu.Unlock()
		return errors.New("enet: not connected")
	}
	if uint32(channelID) >= c.channelCount {
		c.mu.Unlock()
		return fmt.Errorf("enet: invalid channel %d", channelID)
	}
	c.outgoingRelSeq[channelID]++
	seq := c.outgoingRelSeq[channelID]

	hdr := c.buildHeader(true)
	pkt := make([]byte, len(hdr)+6+len(payload))
	copy(pkt, hdr)
	cmd := pkt[len(hdr):]
	cmd[0] = enetCmdSendReliable | enetCmdFlagAck
	cmd[1] = channelID
	binary.BigEndian.PutUint16(cmd[2:4], seq)
	binary.BigEndian.PutUint16(cmd[4:6], uint16(len(payload)))
	copy(cmd[6:], payload)

	key := (uint32(channelID) << 16) | uint32(seq)
	c.pendingReliable[key] = &enetPendingReliable{
		channelID: channelID,
		seq:       seq,
		packet:    pkt,
		sentAt:    time.Now(),
	}
	c.mu.Unlock()

	_, err := c.conn.Write(pkt)
	return err
}

func (c *enetClient) sendUnsequenced(channelID uint8, payload []byte) error {
	c.mu.Lock()
	if c.closed || !c.connected {
		c.mu.Unlock()
		return errors.New("enet: not connected")
	}
	c.outgoingUnseqGrp++
	grp := c.outgoingUnseqGrp
	hdr := c.buildHeader(false)
	c.mu.Unlock()

	pkt := make([]byte, len(hdr)+8+len(payload))
	copy(pkt, hdr)
	cmd := pkt[len(hdr):]
	cmd[0] = enetCmdSendUnsequenced
	cmd[1] = channelID
	binary.BigEndian.PutUint16(cmd[2:4], 0)
	binary.BigEndian.PutUint16(cmd[4:6], grp)
	binary.BigEndian.PutUint16(cmd[6:8], uint16(len(payload)))
	copy(cmd[8:], payload)

	_, err := c.conn.Write(pkt)
	return err
}

func (c *enetClient) sendAck(channelID uint8, receivedRelSeq uint16, receivedSentTime uint16) {
	c.mu.Lock()
	hdr := c.buildHeader(false)
	c.mu.Unlock()

	pkt := make([]byte, len(hdr)+8)
	copy(pkt, hdr)
	cmd := pkt[len(hdr):]
	cmd[0] = enetCmdAcknowledge
	cmd[1] = channelID
	binary.BigEndian.PutUint16(cmd[2:4], receivedRelSeq)
	binary.BigEndian.PutUint16(cmd[4:6], receivedRelSeq)
	binary.BigEndian.PutUint16(cmd[6:8], receivedSentTime)
	_, _ = c.conn.Write(pkt)
}

func (c *enetClient) recvLoop() {
	buf := make([]byte, 4096)
	for {
		select {
		case <-c.done:
			return
		default:
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, err := c.conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return
		}
		if n < 2 {
			continue
		}
		c.handlePacket(buf[:n])
	}
}

func (c *enetClient) handlePacket(pkt []byte) {
	peerField := binary.BigEndian.Uint16(pkt[0:2])
	hasSentTime := (peerField & enetHdrFlagSentTime) != 0
	offset := 2
	var sentTime uint16
	if hasSentTime {
		if len(pkt) < 4 {
			return
		}
		sentTime = binary.BigEndian.Uint16(pkt[2:4])
		offset = 4
	}

	for offset+4 <= len(pkt) {
		cmdByte := pkt[offset]
		cmdType := cmdByte & enetCmdMask
		needAck := (cmdByte & enetCmdFlagAck) != 0
		channelID := pkt[offset+1]
		relSeq := binary.BigEndian.Uint16(pkt[offset+2 : offset+4])

		if needAck {
			c.sendAck(channelID, relSeq, sentTime)
		}

		switch cmdType {
		case enetCmdAcknowledge:
			if offset+8 > len(pkt) {
				return
			}
			ackSeq := binary.BigEndian.Uint16(pkt[offset+4 : offset+6])
			c.mu.Lock()
			delete(c.pendingReliable, (uint32(channelID)<<16)|uint32(ackSeq))
			c.mu.Unlock()
			offset += 8

		case enetCmdVerifyConnect:
			if offset+44 > len(pkt) {
				return
			}
			outPeer := binary.BigEndian.Uint16(pkt[offset+4 : offset+6])
			inSess := pkt[offset+6]
			outSess := pkt[offset+7]
			c.mu.Lock()
			c.outgoingPeerID = outPeer
			c.incomingSession = inSess
			c.outgoingSession = outSess
			c.connected = true
			c.mu.Unlock()
			select {
			case c.connectAckCh <- nil:
			default:
			}
			offset += 44

		case enetCmdDisconnect:
			offset += 8
			c.close()
			return

		case enetCmdPing:
			offset += 4

		case enetCmdSendReliable:
			if offset+6 > len(pkt) {
				return
			}
			dataLen := int(binary.BigEndian.Uint16(pkt[offset+4 : offset+6]))
			offset += 6 + dataLen

		case enetCmdSendUnsequenced:
			if offset+8 > len(pkt) {
				return
			}
			dataLen := int(binary.BigEndian.Uint16(pkt[offset+6 : offset+8]))
			offset += 8 + dataLen

		default:
			return
		}
	}
}

func (c *enetClient) retransmitLoop() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.Lock()
			if c.closed {
				c.mu.Unlock()
				return
			}
			now := time.Now()
			var resend [][]byte
			for _, p := range c.pendingReliable {
				if now.Sub(p.sentAt) > 300*time.Millisecond {
					p.sentAt = now
					resend = append(resend, p.packet)
				}
			}
			c.mu.Unlock()
			for _, pkt := range resend {
				_, _ = c.conn.Write(pkt)
			}
		}
	}
}

func (c *enetClient) close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	wasConnected := c.connected
	c.connected = false
	hdr := c.buildHeader(false)
	c.mu.Unlock()

	if wasConnected {
		pkt := make([]byte, len(hdr)+8)
		copy(pkt, hdr)
		pkt[len(hdr)] = enetCmdDisconnect
		pkt[len(hdr)+1] = 0xFF
		_, _ = c.conn.Write(pkt)
	}
	close(c.done)
	_ = c.conn.Close()
}
