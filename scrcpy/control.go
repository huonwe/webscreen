package scrcpy

import (
	"encoding/binary"
	"log"
)

func (da *DataAdapter) SendTouchEvent(e TouchEvent) {
	if da.controlConn == nil {
		return
	}

	// 1. 预分配一个固定大小的字节切片 (Scrcpy 协议触摸包固定 28 字节)
	// 这里的 buf 可以在对象池(sync.Pool)里复用，进一步减少 GC
	buf := make([]byte, 28)

	// 2. 使用 Put 系列函数直接填充内存，速度极快
	buf[0] = TYPE_INJECT_TOUCH_EVENT                   // Type
	buf[1] = e.Action                                  // Action
	binary.BigEndian.PutUint64(buf[2:10], e.PointerID) // PointerID (8 bytes)
	binary.BigEndian.PutUint32(buf[10:14], e.PosX)     // PosX (4 bytes)
	binary.BigEndian.PutUint32(buf[14:18], e.PosY)     // PosY (4 bytes)
	binary.BigEndian.PutUint16(buf[18:20], e.Width)    // Width (2 bytes)
	binary.BigEndian.PutUint16(buf[20:22], e.Height)   // Height (2 bytes)
	binary.BigEndian.PutUint16(buf[22:24], e.Pressure) // Pressure (2 bytes)
	binary.BigEndian.PutUint32(buf[24:28], e.Buttons)  // Buttons (4 bytes)

	// 3. 一次性发送
	_, err := da.controlConn.Write(buf)
	if err != nil {
		log.Printf("Error sending touch event: %v\n", err)
	}
}

// func (da *DataAdapter) SendKeyEvent(e KeyEvent) {
// 	if da.controlConn == nil {
// 		return
// 	}

// 	buf := make([]byte, 6)

// 	buf[0] = TYPE_INJECT_KEY_EVENT                  // Type
// 	buf[1] = e.Action                               // Action
// 	binary.BigEndian.PutUint32(buf[2:6], e.KeyCode) // KeyCode (4 bytes)

// 	_, err := da.controlConn.Write(buf)
// 	if err != nil {
// 		log.Printf("Error sending key event: %v\n", err)
// 	}
// }

func (da *DataAdapter) RotateDevice() {
	if da.controlConn == nil {
		return
	}
	log.Println("Sending Rotate Device command...")
	msg := []byte{TYPE_ROTATE_DEVICE}
	_, err := da.controlConn.Write(msg)
	if err != nil {
		log.Printf("Error sending rotate command: %v\n", err)
	}
}

func (da *DataAdapter) RequestKeyFrame() error {
	if da.controlConn == nil {
		return nil
	}
	log.Println("⚡ Sending Request KeyFrame (Type 99)...")
	msg := []byte{ControlMsgTypeReqIDR}
	_, err := da.controlConn.Write(msg)
	if err != nil {
		log.Printf("Error sending keyframe request: %v\n", err)
		return err
	}
	return nil
}
