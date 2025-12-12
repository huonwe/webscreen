package scrcpy

import (
	"io"
	"log"
	"net"
)

func ReadDeviceMeta(conn net.Conn) (string, error) {
	// 1. Device Name (64 bytes)
	nameBuf := make([]byte, 64)
	_, err := io.ReadFull(conn, nameBuf)
	if err != nil {
		return "", err
	}
	deviceName := string(nameBuf)
	return deviceName, nil
}

func ReadVideoMeta(conn net.Conn) {
	// 1. Device Name (64 bytes) - 你已经读过了
	// ...

	// 2. Codec ID (4 bytes)
	// 3. Width (4 bytes)
	// 4. Height (4 bytes)
	// 共 12 字节的额外元数据
	metaBuf := make([]byte, 12)
	if _, err := io.ReadFull(conn, metaBuf); err != nil {
		log.Println("Failed to read metadata:", err)
		return
	}
	log.Printf("Metadata read. Ready for video stream.")
}
