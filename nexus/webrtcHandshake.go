package nexus

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/pion/interceptor"
	pionSDP "github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v5"
)

func createPeerConnection(cfg ConnectionConfig) (*webrtc.PeerConnection, error) {
	if cfg.VideoCodec != "h264" && cfg.VideoCodec != "h265" && cfg.VideoCodec != "av1" {
		return nil, errors.New("WebRTC: Unsupported video codec: " + cfg.VideoCodec)
	}
	sdp := cfg.Extra["sdp"]
	if sdp == "" {
		return nil, errors.New("SDP is required for WebRTC connection")
	}
	sdpOffersH265 := func(sdp string) bool {
		lower := strings.ToLower(sdp)
		return strings.Contains(lower, "h265") || strings.Contains(lower, "hevc")
	}

	if strings.EqualFold(cfg.VideoCodec, "h265") && !sdpOffersH265(sdp) {
		log.Printf("[webscreen] browser SDP does not offer H.265 (HEVC)")
		return nil, errors.New("This browser does not support H.265 (HEVC); streaming with H.264 instead.")
	}

	videoMimeType, err := getMimeTypeFromString(cfg.VideoCodec)
	if err != nil {
		return nil, err
	}
	audioMimeType, err := getMimeTypeFromString(cfg.AudioCodec)
	if err != nil {
		return nil, err
	}
	if audioMimeType != "audio/opus" {
		return nil, errors.New("WebRTC: Unsupported audio codec: " + cfg.AudioCodec)
	}
	mimeTypes := []string{videoMimeType, audioMimeType}
	m := createMediaEngine(mimeTypes)
	err = m.RegisterHeaderExtension(
		webrtc.RTPHeaderExtensionCapability{URI: pionSDP.TransportCCURI},
		webrtc.RTPCodecTypeVideo,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register header extension: %v", err)
	}
	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, fmt.Errorf("failed to register default interceptors: %v", err)
	}
	settingEngine := webrtc.SettingEngine{}
	err = settingEngine.SetEphemeralUDPPortRange(UDP_PORT_START, UDP_PORT_END)
	if err != nil {
		return nil, fmt.Errorf("failed to set ephemeral UDP port range: %v", err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i), webrtc.WithSettingEngine(settingEngine))
	// Configure ICE servers (STUN) for NAT traversal
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
	// Create PeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create PeerConnection: %v", err)
	}

	return peerConnection, nil
}
