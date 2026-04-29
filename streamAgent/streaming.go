package sagent

import (
	"log"
	"time"
	"webscreen/sdriver"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
)

func (sa *Agent) ServeVideoStream() {
	if sa.videoCh == nil {
		log.Println("[Agent] Video channel is nil, skipping video streaming")
		sa.controlCh <- sdriver.TextMsgEvent{Msg: "Video channel is nil, cannot stream video."}
		return
	}

	// 初始化打包器 (Pion 内部自带的工具)
	codec := sa.videoTrack.Codec().MimeType
	var payloader rtp.Payloader
	switch codec {
	case "video/H264":
		payloader = &codecs.H264Payloader{}
	case "video/H265":
		payloader = &codecs.H265Payloader{}
	default:
		log.Printf("Unsupported video codec: %s", codec)
		return
	}
	packetizer := rtp.NewPacketizer(
		1200, // MTU 大小，通常 1200 左右很安全
		0,    // Payload Type，Pion 会自动覆盖它，填 0 即可
		0,    // SSRC (TrackLocalStaticRTP 会自动覆盖它，填 0 即可)
		payloader,
		rtp.NewRandomSequencer(), // 随机序列号生成器
		90000,                    // 视频的基准时钟频率 (WebRTC 规定视频固定为 90kHz)
	)
	// var lastPTS uint64 = 0
	var exactRtpTimestamp uint32 = 0
	for vBox := range sa.videoCh {
		switch sa.useLocalTimestamp {
		case true:
			elapsedUs := time.Since(sa.startTime).Microseconds()
			exactRtpTimestamp = uint32((elapsedUs * 90) / 1000) // 换算为 90kHz 的 RTP 时间戳
		case false:
			exactRtpTimestamp = uint32((vBox.PTS * 90) / 1000)
		}

		packets := packetizer.Packetize(vBox.Data, 1)
		for _, p := range packets {
			p.Timestamp = exactRtpTimestamp
			if err := sa.videoTrack.WriteRTP(p); err != nil {
				log.Printf("Failed to write video RTP packet: %v", err)
				return
			}
		}
	}
}

func (sa *Agent) ServeAudioStream() {
	if sa.audioCh == nil {
		return
	}
	packetizer := rtp.NewPacketizer(1200, 0, 0, &codecs.OpusPayloader{}, rtp.NewRandomSequencer(), 48000)

	var currentAudioRTP uint32 = 0
	var isInit bool

	for aBox := range sa.audioCh {
		// 计算从 Agent Start() 到现在，本地服务器流逝的真实时间
		elapsedUs := time.Since(sa.startTime).Microseconds()

		// 本地真实时间换算成的理论音频 RTP
		realLocalRTP := uint32((elapsedUs * 48) / 1000)

		if !isInit {
			currentAudioRTP = realLocalRTP
			isInit = true
		} else {
			// 【防漂移机制】：计算我们的平滑时间与本地真实时钟的偏差
			diff := int64(realLocalRTP) - int64(currentAudioRTP)

			// 如果音频帧积压或晚到了超过 100ms (4800采样点)，强行重置对齐
			if diff > 4800 || diff < -4800 {
				log.Printf("Local audio drift corrected: %d samples", diff)
				currentAudioRTP = realLocalRTP // 猛拉风筝线
			}
		}

		packets := packetizer.Packetize(aBox.Data, 1)
		for _, p := range packets {
			p.Timestamp = currentAudioRTP
			if err := sa.audioTrack.WriteRTP(p); err != nil {
				log.Printf("Failed to write audio RTP packet: %v", err)
				return
			}
		}

		// 严格递增 960 (20ms)
		currentAudioRTP += 960
	}
}
