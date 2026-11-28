package gamestream

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"scrmux/consts"
	"strconv"
	"strings"
	"time"
)

const (
	ssEncControlV2 = 0x01
	ssEncVideo     = 0x02
	ssEncAudio     = 0x04

	nvffBase            = 0x07
	nvffAudioEncryption = 0x20
	nvffRiEncryption    = 0x80

	encryptedRTSPBit    = uint32(0x80000000)
	encRTSPHeaderLength = 24 // 4B typeAndLength + 4B seq + 16B tag
)

type rtspResult struct {
	audioPort          int
	videoPort          int
	controlPort        int
	audioPingPayload   [16]byte
	hasAudioPing       bool
	videoPingPayload   [16]byte
	hasVideoPing       bool
	controlConnectData uint32
	encryptionEnabled  uint32
	audioEncrypted     bool
	videoPacketSize    int
}

type rtspMessage struct {
	statusCode int
	headers    map[string]string
	body       string
}

type rtspHeader struct {
	key string
	val string
}

type rtspClient struct {
	dialHost      string
	hostHeader    string
	port          string
	targetURL     string
	clientVersion int
	cseq          int
	sessionID     string
	encrypted     bool
	encSeq        uint32
	aead          cipher.AEAD
}

func performRTSPHandshake(hs *httpSession, cfg *streamConfig) (*rtspResult, error) {
	log.Printf("[gamestream] starting RTSP handshake (sessionUrl0=%q, targetIP=%s)", hs.sessionURL, cfg.ip)
	u, err := url.Parse(hs.sessionURL)
	if err != nil {
		return nil, fmt.Errorf("gamestream: parse sessionUrl %q: %w", hs.sessionURL, err)
	}
	// Dial the configured host IP (cfg.ip), but send sessionUrl0's hostname in the RTSP Host header
	// because servers like Wolf use that synthetic IP in sessionUrl0 to match RTSP SETUP/ANNOUNCE/PLAY requests.
	dialHost := cfg.ip
	hostHeader := u.Hostname()
	if hostHeader == "" {
		hostHeader = cfg.ip
	}
	port := u.Port()
	if port == "" {
		port = "48010"
	}

	encrypted := strings.EqualFold(u.Scheme, "rtspenc")
	var aead cipher.AEAD
	if encrypted {
		blk, err := aes.NewCipher(hs.rikey[:])
		if err != nil {
			return nil, err
		}
		aead, err = cipher.NewGCM(blk)
		if err != nil {
			return nil, err
		}
	}

	clientVer := 14
	switch {
	case hs.appVersion[0] == 3:
		clientVer = 10
	case hs.appVersion[0] == 4:
		clientVer = 11
	case hs.appVersion[0] == 5:
		clientVer = 12
	case hs.appVersion[0] == 6:
		clientVer = 13
	}

	targetURL := hs.sessionURL
	if targetURL == "" {
		scheme := "rtsp"
		if encrypted {
			scheme = "rtspenc"
		}
		targetURL = fmt.Sprintf("%s://%s:%s", scheme, hostHeader, port)
	}

	rc := &rtspClient{
		dialHost:      dialHost,
		hostHeader:    hostHeader,
		port:          port,
		targetURL:     targetURL,
		clientVersion: clientVer,
		encrypted:     encrypted,
		aead:          aead,
	}

	res := &rtspResult{
		audioPort:       48000,
		videoPort:       47998,
		controlPort:     47999,
		videoPacketSize: 1024,
	}

	// 1. OPTIONS
	if resp, err := rc.do("OPTIONS", rc.targetURL, nil, ""); err != nil || resp.statusCode != 200 {
		return nil, fmt.Errorf("rtsp OPTIONS failed: err=%v status=%v", err, statusOf(resp))
	}

	// 2. DESCRIBE
	descResp, err := rc.do("DESCRIBE", rc.targetURL, []rtspHeader{
		{"Accept", "application/sdp"},
		{"If-Modified-Since", "Thu, 01 Jan 1970 00:00:00 GMT"},
	}, "")
	if err != nil || descResp.statusCode != 200 {
		return nil, fmt.Errorf("rtsp DESCRIBE failed: err=%v status=%v", err, statusOf(descResp))
	}
	log.Printf("[gamestream] RTSP DESCRIBE ok (sdpLen=%d)", len(descResp.body))

	encSupported := parseSDPAttrUint(descResp.body, "x-ss-general.encryptionSupported")
	encRequested := parseSDPAttrUint(descResp.body, "x-ss-general.encryptionRequested")
	if encSupported&ssEncControlV2 != 0 {
		res.encryptionEnabled |= ssEncControlV2
	}
	if encRequested&ssEncVideo != 0 {
		res.encryptionEnabled |= ssEncVideo
	}
	if encRequested&ssEncAudio != 0 {
		res.encryptionEnabled |= ssEncAudio
	}

	// 3. SETUP audio
	audioSetup, err := rc.setup("streamid=audio/0/0")
	if err != nil || audioSetup.statusCode != 200 {
		return nil, fmt.Errorf("rtsp SETUP audio failed: err=%v status=%v", err, statusOf(audioSetup))
	}
	if sess := audioSetup.headers["session"]; sess != "" {
		if idx := strings.IndexByte(sess, ';'); idx >= 0 {
			sess = sess[:idx]
		}
		rc.sessionID = strings.TrimSpace(sess)
	}
	if p := parseServerPort(audioSetup.headers["transport"]); p > 0 {
		res.audioPort = p
	}
	if ping := audioSetup.headers["x-ss-ping-payload"]; len(ping) == 16 {
		copy(res.audioPingPayload[:], ping)
		res.hasAudioPing = true
	}

	// 4. SETUP video
	videoSetup, err := rc.setup("streamid=video/0/0")
	if err != nil || videoSetup.statusCode != 200 {
		return nil, fmt.Errorf("rtsp SETUP video failed: err=%v status=%v", err, statusOf(videoSetup))
	}
	if p := parseServerPort(videoSetup.headers["transport"]); p > 0 {
		res.videoPort = p
	}
	if ping := videoSetup.headers["x-ss-ping-payload"]; len(ping) == 16 {
		copy(res.videoPingPayload[:], ping)
		res.hasVideoPing = true
	}

	// 5. SETUP control
	ctrlSetup, err := rc.setup("streamid=control/13/0")
	if err != nil || ctrlSetup.statusCode != 200 {
		return nil, fmt.Errorf("rtsp SETUP control failed: err=%v status=%v", err, statusOf(ctrlSetup))
	}
	if p := parseServerPort(ctrlSetup.headers["transport"]); p > 0 {
		res.controlPort = p
	}
	if cd := ctrlSetup.headers["x-ss-connect-data"]; cd != "" {
		if v, err := strconv.ParseUint(cd, 0, 32); err == nil {
			res.controlConnectData = uint32(v)
		}
	}

	// 6. ANNOUNCE
	if res.encryptionEnabled&ssEncVideo != 0 {
		res.videoPacketSize -= 32 // ENC_VIDEO_HEADER size
	}
	if res.encryptionEnabled&ssEncAudio != 0 {
		res.audioEncrypted = true
	}
	sdp := buildAnnounceSDP(hs, cfg, res)
	annResp, err := rc.do("ANNOUNCE", "streamid=control/13/0", []rtspHeader{
		{"Session", rc.sessionID},
		{"Content-type", "application/sdp"},
		{"Content-length", strconv.Itoa(len(sdp))},
	}, sdp)
	if err != nil || annResp.statusCode != 200 {
		return nil, fmt.Errorf("rtsp ANNOUNCE failed: err=%v status=%v", err, statusOf(annResp))
	}

	// 7. PLAY
	playResp, err := rc.do("PLAY", "/", []rtspHeader{
		{"Session", rc.sessionID},
	}, "")
	if err != nil || playResp.statusCode != 200 {
		return nil, fmt.Errorf("rtsp PLAY failed: err=%v status=%v", err, statusOf(playResp))
	}

	return res, nil
}

func (rc *rtspClient) setup(target string) (*rtspMessage, error) {
	var hdrs []rtspHeader
	if rc.sessionID != "" {
		hdrs = append(hdrs, rtspHeader{"Session", rc.sessionID})
	}
	hdrs = append(hdrs,
		rtspHeader{"Transport", "unicast;X-GS-ClientPort=50000-50001"},
		rtspHeader{"If-Modified-Since", "Thu, 01 Jan 1970 00:00:00 GMT"},
	)
	return rc.do("SETUP", target, hdrs, "")
}

func (rc *rtspClient) do(method, target string, headers []rtspHeader, body string) (*rtspMessage, error) {
	rc.cseq++
	addr := net.JoinHostPort(rc.dialHost, rc.port)

	var conn net.Conn
	var err error
	// Retry ECONNREFUSED briefly right after /launch
	deadline := time.Now().Add(8 * time.Second)
	for {
		conn, err = net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(250 * time.Millisecond)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %s RTSP/1.0\r\n", method, target)
	fmt.Fprintf(&sb, "CSeq: %d\r\n", rc.cseq)
	fmt.Fprintf(&sb, "X-GS-ClientVersion: %d\r\n", rc.clientVersion)
	fmt.Fprintf(&sb, "Host: %s\r\n", rc.hostHeader)
	for _, h := range headers {
		fmt.Fprintf(&sb, "%s: %s\r\n", h.key, h.val)
	}
	sb.WriteString("\r\n")
	if body != "" {
		sb.WriteString(body)
	}

	wire := []byte(sb.String())
	if rc.encrypted {
		rc.encSeq++
		wire = sealRTSPMessage(rc.aead, rc.encSeq, wire)
	}

	if _, err := conn.Write(wire); err != nil {
		return nil, err
	}

	// 1. 加密模式：先按 4 字节长度头精确读满整帧密文，解密后再解析
	if rc.encrypted {
		raw, err := readEncryptedRTSPFrame(conn)
		if err != nil {
			log.Printf("[gamestream] RTSP %s %s read encrypted failed: %v", method, target, err)
			return nil, err
		}
		plain, err := unsealRTSPMessage(rc.aead, raw)
		if err != nil {
			return nil, err
		}
		return parseRTSPResponse(bytes.NewReader(plain))
	}

	// 2. 明文模式：直接从 conn 流式读取 Header + Content-Length Body，不等待 EOF
	resp, err := parseRTSPResponse(conn)
	if err != nil {
		log.Printf("[gamestream] RTSP %s %s read failed: %v", method, target, err)
		return nil, err
	}
	return resp, nil
}
func sealRTSPMessage(aead cipher.AEAD, seq uint32, plaintext []byte) []byte {
	var iv [12]byte
	binary.LittleEndian.PutUint32(iv[0:4], seq)
	iv[10] = 'C'
	iv[11] = 'R'

	sealed := aead.Seal(nil, iv[:], plaintext, nil)
	ctLen := len(plaintext)
	ct := sealed[:ctLen]
	tag := sealed[ctLen:]

	out := make([]byte, encRTSPHeaderLength+ctLen)
	binary.BigEndian.PutUint32(out[0:4], encryptedRTSPBit|uint32(ctLen))
	binary.BigEndian.PutUint32(out[4:8], seq)
	copy(out[8:24], tag)
	copy(out[24:], ct)
	return out
}

func unsealRTSPMessage(aead cipher.AEAD, raw []byte) ([]byte, error) {
	if len(raw) <= encRTSPHeaderLength {
		return nil, fmt.Errorf("encrypted rtsp response too short (%d bytes)", len(raw))
	}
	typeAndLen := binary.BigEndian.Uint32(raw[0:4])
	if typeAndLen&encryptedRTSPBit == 0 {
		return nil, fmt.Errorf("expected encrypted rtsp response")
	}
	seq := binary.BigEndian.Uint32(raw[4:8])
	tag := raw[8:24]
	ct := raw[24:]

	var iv [12]byte
	binary.LittleEndian.PutUint32(iv[0:4], seq)
	iv[10] = 'H'
	iv[11] = 'R'

	sealed := make([]byte, len(ct)+len(tag))
	copy(sealed, ct)
	copy(sealed[len(ct):], tag)
	return aead.Open(nil, iv[:], sealed, nil)
}

func readEncryptedRTSPFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	typeAndLen := binary.BigEndian.Uint32(hdr[:])
	if typeAndLen&encryptedRTSPBit == 0 {
		return nil, fmt.Errorf("expected encrypted rtsp response, got header 0x%08x", typeAndLen)
	}
	ctLen := int(typeAndLen &^ encryptedRTSPBit)
	if ctLen <= 0 || ctLen > 1024*1024 {
		return nil, fmt.Errorf("invalid encrypted rtsp payload length: %d", ctLen)
	}

	raw := make([]byte, encRTSPHeaderLength+ctLen)
	copy(raw[:4], hdr[:])
	if _, err := io.ReadFull(r, raw[4:]); err != nil {
		return nil, err
	}
	return raw, nil
}

func parseRTSPResponse(r io.Reader) (*rtspMessage, error) {
	br := bufio.NewReader(r)

	statusLine, err := br.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read rtsp status line: %w", err)
	}
	statusLine = strings.TrimRight(statusLine, "\r\n")
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid rtsp status line: %q", statusLine)
	}
	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid rtsp status code: %w", err)
	}

	hdrs := make(map[string]string)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read rtsp header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// 读到空行 \r\n\r\n，Header 结束
			break
		}
		if colon := strings.IndexByte(line, ':'); colon > 0 {
			k := strings.ToLower(strings.TrimSpace(line[:colon]))
			v := strings.TrimSpace(line[colon+1:])
			hdrs[k] = v
		}
	}

	var bodyStr string
	if clStr, ok := hdrs["content-length"]; ok {
		cl, err := strconv.Atoi(clStr)
		if err != nil || cl < 0 {
			return nil, fmt.Errorf("invalid Content-Length %q: %w", clStr, err)
		}
		if cl > 0 {
			bodyBuf := make([]byte, cl)
			if _, err := io.ReadFull(br, bodyBuf); err != nil {
				return nil, fmt.Errorf("read rtsp body (%d bytes): %w", cl, err)
			}
			bodyStr = string(bodyBuf)
		}
	}

	return &rtspMessage{
		statusCode: code,
		headers:    hdrs,
		body:       bodyStr,
	}, nil
}

func parseServerPort(transport string) int {
	for _, part := range strings.Split(transport, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "server_port=") {
			val := strings.TrimPrefix(part, "server_port=")
			if dash := strings.IndexByte(val, '-'); dash >= 0 {
				val = val[:dash]
			}
			if p, err := strconv.Atoi(val); err == nil && p > 0 {
				return p
			}
		}
	}
	return 0
}

func parseSDPAttrUint(sdp, key string) uint32 {
	prefix := "a=" + key + ":"
	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if v, err := strconv.ParseUint(val, 10, 32); err == nil {
				return uint32(v)
			}
		}
	}
	return 0
}

func statusOf(m *rtspMessage) int {
	if m == nil {
		return -1
	}
	return m.statusCode
}

func buildAnnounceSDP(hs *httpSession, cfg *streamConfig, res *rtspResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "v=0\r\no=android 0 %d IN IPv4 %s\r\ns=NVIDIA Streaming Client\r\n", 14, cfg.ip)

	attr := func(k, v string) {
		fmt.Fprintf(&sb, "a=%s:%s \r\n", k, v)
	}

	if hs.isSunshine {
		attr("x-ml-general.featureFlags", "3")
		attr("x-ss-general.encryptionEnabled", strconv.FormatUint(uint64(res.encryptionEnabled), 10))
		attr("x-ss-video[0].chromaSamplingType", "0")
		attr("x-ss-video[0].intraRefresh", "0")
	}

	attr("x-nv-video[0].clientViewportWd", strconv.Itoa(cfg.width))
	attr("x-nv-video[0].clientViewportHt", strconv.Itoa(cfg.height))
	attr("x-nv-video[0].maxFPS", strconv.Itoa(cfg.fps))
	attr("x-nv-video[0].packetSize", strconv.Itoa(res.videoPacketSize))
	attr("x-nv-video[0].rateControlMode", "4")
	attr("x-nv-video[0].timeoutLengthMs", "7000")
	attr("x-nv-video[0].framesWithInvalidRefThreshold", "0")

	// Print width and height for debugging
	fmt.Printf("[gamestream] client viewport: %dx%d\n", cfg.width, cfg.height)
	adjBitrate := int(float64(cfg.bitrate) * 0.80)
	if adjBitrate > 100000 {
		adjBitrate = 100000
	}
	if adjBitrate < 500 {
		adjBitrate = 500
	}
	adjStr := strconv.Itoa(adjBitrate)
	attr("x-nv-video[0].initialBitrateKbps", adjStr)
	attr("x-nv-video[0].initialPeakBitrateKbps", adjStr)
	attr("x-nv-vqos[0].bw.minimumBitrateKbps", adjStr)
	attr("x-nv-vqos[0].bw.maximumBitrateKbps", adjStr)
	if hs.isSunshine {
		attr("x-ml-video.configuredBitrateKbps", strconv.Itoa(cfg.bitrate))
	}

	attr("x-nv-vqos[0].fec.enable", "1")
	attr("x-nv-vqos[0].videoQualityScoreUpdateTime", "5000")
	attr("x-nv-vqos[0].qosTrafficType", "5")
	attr("x-nv-aqos.qosTrafficType", "4")

	ff := uint32(nvffBase | nvffRiEncryption)
	if res.audioEncrypted {
		ff |= nvffAudioEncryption
	}
	attr("x-nv-general.featureFlags", strconv.FormatUint(uint64(ff), 10))
	attr("x-nv-general.useReliableUdp", "13")
	attr("x-nv-vqos[0].fec.minRequiredFecPackets", "2")
	attr("x-nv-vqos[0].bllFec.enable", "0")
	attr("x-nv-vqos[0].drc.enable", "0")
	attr("x-nv-general.enableRecoveryMode", "0")
	attr("x-nv-video[0].videoEncoderSlicesPerFrame", "1")

	if cfg.codec == consts.CODEC_TYPE_H265 {
		attr("x-nv-clientSupportHevc", "1")
		attr("x-nv-vqos[0].bitStreamFormat", "1")
	} else {
		attr("x-nv-clientSupportHevc", "0")
		attr("x-nv-vqos[0].bitStreamFormat", "0")
	}

	attr("x-nv-video[0].dynamicRangeMode", "0")
	attr("x-nv-video[0].maxNumReferenceFrames", "1")
	attr("x-nv-video[0].clientRefreshRateX100", strconv.Itoa(cfg.fps*100))

	attr("x-nv-audio.surround.numChannels", "2")
	attr("x-nv-audio.surround.channelMask", "3")
	attr("x-nv-audio.surround.enable", "0")
	attr("x-nv-audio.surround.AudioQuality", "0")
	attr("x-nv-aqos.packetDuration", "20")
	attr("x-nv-video[0].encoderCscMode", "0")

	fmt.Fprintf(&sb, "t=0 0\r\nm=video %d  \r\n", res.videoPort)
	return sb.String()
}
