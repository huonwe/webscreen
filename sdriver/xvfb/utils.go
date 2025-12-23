package linuxXvfbDriver

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"os"
)

func (da *LinuxDriver) readVideoMeta(conn net.Conn) error {
	// Width (4 bytes)
	// Height (4 bytes)
	// Codec 已经在外面读取过了，用于确认是哪个通道
	metaBuf := make([]byte, 8)
	if _, err := io.ReadFull(conn, metaBuf); err != nil {
		log.Println("Failed to read metadata:", err)
		return err
	}
	// 解析元数据
	// da.mediaMeta.Width = binary.BigEndian.Uint32(metaBuf[0:4])
	// da.mediaMeta.Height = binary.BigEndian.Uint32(metaBuf[4:8])

	return nil
}

func readHeader(buf []byte, header *Header) error {
	if len(buf) < 12 {
		return io.ErrUnexpectedEOF
	}
	header.PTS = binary.BigEndian.Uint64(buf[0:8])
	header.Size = binary.BigEndian.Uint32(buf[8:12])
	return nil
}

func GetTMPDir() string {
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	return tmpDir
}
