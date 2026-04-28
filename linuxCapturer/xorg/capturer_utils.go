package main

import (
	"bytes"
	"log"
	"net"
	"os/exec"
	"strings"
)

func WaitTCP(port string) net.Conn {
	var err error
	var conn net.Conn
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Println("Failed to start video listener:", err)
	}
	conn, err = listener.Accept()
	if err != nil {
		log.Println("Failed to accept connection:", err)
	}
	listener.Close()
	log.Println("TCP connection established:", port)

	return conn
}

func GetBestH264Encoder() string {
	if hasEncoder("h264_rkmpp") {
		return "h264_rkmpp"
	}
	return "libx264"
}

func GetBestHEVCEncoder() string {
	if hasEncoder("hevc_rkmpp") {
		return "hevc_rkmpp"
	}
	if hasEncoder("hevc_nvenc") {
		return "hevc_nvenc"
	}
	if hasEncoder("hevc_qsv") {
		return "hevc_qsv"
	}
	if hasEncoder("hevc_amf") {
		return "hevc_amf"
	}
	if hasEncoder("hevc_videotoolbox") {
		return "hevc_videotoolbox"
	}
	if hasEncoder("hevc_mediacodec") {
		return "hevc_mediacodec"
	}
	if hasEncoder("hevc_vaapi") {
		return "hevc_vaapi"
	}
	return "libx265"
}

func hasEncoder(name string) bool {
	cmd := exec.Command("ffmpeg", "-encoders")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), name)
}

func splitNALU(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	start := 0
	firstStart := bytes.Index(data, []byte{0, 0, 1})
	if firstStart == -1 {
		if atEOF {
			return len(data), nil, nil
		}
		return 0, nil, nil
	}

	if firstStart > 0 && data[firstStart-1] == 0 {
		start = firstStart - 1
	} else {
		start = firstStart
	}

	nextStart := bytes.Index(data[start+3:], []byte{0, 0, 1})
	if nextStart != -1 {
		end := start + 3 + nextStart
		if data[end-1] == 0 {
			end--
		}
		return end, data[start:end], nil
	}

	if atEOF {
		return len(data), data[start:], nil
	}

	if start > 0 {
		return start, nil, nil
	}

	return 0, nil, nil
}

func getNalType(data []byte) byte {
	for i := 0; i < len(data)-1; i++ {
		if data[i] == 0 && data[i+1] == 1 {
			if i+2 < len(data) {
				return data[i+2] & 0x1F
			}
		}
	}
	return 0
}
