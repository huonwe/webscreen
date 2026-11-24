package gamestream

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
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

type rtspClient struct {
	host          string
	port          string
	targetURL     string
	clientVersion int
	cseq          int
	sessionID     string
}

func performRTSPHandshake(hs *httpSession, cfg *streamConfig) (*rtspResult, error) {
	u, err := url.Parse(hs.sessionURL)
	if err != nil {
		return nil, fmt.Errorf("gamestream: parse sessionUrl %q: %w", hs.sessionURL, err)
	}
	// Always dial the configured host IP (cfg.ip) rather than the hostname in sessionUrl0,
	// because servers behind NAT/Docker (such as Wolf) may report a WAN or container IP there.
	host := cfg.ip
	port := u.Port()
	if port == "" {
		port = "48010"
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

	rc := &rtspClient{
		host:          host,
		port:          port,
		targetURL:     fmt.Sprintf("rtsp://%s:%s", host, port),
		clientVersion: clientVer,
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
	descResp, err := rc.do("DESCRIBE", rc.targetURL, map[string]string{
		"Accept":            "application/sdp",
		"If-Modified-Since": "Thu, 01 Jan 1970 00:00:00 GMT",
	}, "")
	if err != nil || descResp.statusCode != 200 {
		return nil, fmt.Errorf("rtsp DESCRIBE failed: err=%v status=%v", err, statusOf(descResp))
	}

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
	annResp, err := rc.do("ANNOUNCE", "streamid=control/13/0", map[string]string{
		"Session":        rc.sessionID,
		"Content-type":   "application/sdp",
		"Content-length": strconv.Itoa(len(sdp)),
	}, sdp)
	if err != nil || annResp.statusCode != 200 {
		return nil, fmt.Errorf("rtsp ANNOUNCE failed: err=%v status=%v", err, statusOf(annResp))
	}

	// 7. PLAY
	playResp, err := rc.do("PLAY", "/", map[string]string{
		"Session": rc.sessionID,
	}, "")
	if err != nil || playResp.statusCode != 200 {
		return nil, fmt.Errorf("rtsp PLAY failed: err=%v status=%v", err, statusOf(playResp))
	}

	return res, nil
}

func (rc *rtspClient) setup(target string) (*rtspMessage, error) {
	hdrs := map[string]string{
		"Transport":         "unicast;X-GS-ClientPort=50000-50001",
		"If-Modified-Since": "Thu, 01 Jan 1970 00:00:00 GMT",
	}
	if rc.sessionID != "" {
		hdrs["Session"] = rc.sessionID
	}
	return rc.do("SETUP", target, hdrs, "")
}

func (rc *rtspClient) do(method, target string, headers map[string]string, body string) (*rtspMessage, error) {
	rc.cseq++
	addr := net.JoinHostPort(rc.host, rc.port)

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
	fmt.Fprintf(&sb, "Host: %s\r\n", rc.host)
	for k, v := range headers {
		fmt.Fprintf(&sb, "%s: %s\r\n", k, v)
	}
	sb.WriteString("\r\n")
	if body != "" {
		sb.WriteString(body)
	}

	if _, err := conn.Write([]byte(sb.String())); err != nil {
		return nil, err
	}

	raw, err := io.ReadAll(conn)
	if err != nil && len(raw) == 0 {
		return nil, err
	}
	return parseRTSPResponse(raw)
}

func parseRTSPResponse(raw []byte) (*rtspMessage, error) {
	idx := bytes.Index(raw, []byte("\r\n\r\n"))
	var headerPart, bodyPart []byte
	if idx >= 0 {
		headerPart = raw[:idx]
		bodyPart = raw[idx+4:]
	} else {
		headerPart = raw
	}

	scanner := bufio.NewScanner(bytes.NewReader(headerPart))
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty rtsp response")
	}
	statusLine := scanner.Text()
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid rtsp status line: %q", statusLine)
	}
	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid rtsp status code: %w", err)
	}

	hdrs := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if colon := strings.IndexByte(line, ':'); colon > 0 {
			k := strings.ToLower(strings.TrimSpace(line[:colon]))
			v := strings.TrimSpace(line[colon+1:])
			hdrs[k] = v
		}
	}
	return &rtspMessage{
		statusCode: code,
		headers:    hdrs,
		body:       string(bodyPart),
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

	if cfg.codec == "h265" {
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
