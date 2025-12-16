package scrcpy

import (
	"encoding/binary"
	"log"
	"time"
)

func (da *DataAdapter) SendTouchEvent(e TouchEvent) {
	if da.controlConn == nil {
		return
	}
	// log.Printf("sending touch event: %v\n", e)
	// log.Printf("current video width height: %vx%v", da.VideoMeta.Width, da.VideoMeta.Height)
	// 1. 预分配一个固定大小的字节切片 (Scrcpy 协议触摸包固定 28 字节)
	// 这里的 buf 可以在对象池(sync.Pool)里复用，进一步减少 GC
	buf := make([]byte, 32)
	if e.Type != TYPE_INJECT_TOUCH_EVENT {
		log.Printf("Mismatch Event Type: %d\n", e.Type)
		return
	}
	// 2. 使用 Put 系列函数直接填充内存，速度极快
	buf[0] = e.Type                                    // Type
	buf[1] = e.Action                                  // Action
	binary.BigEndian.PutUint64(buf[2:10], e.PointerID) // PointerID (8 bytes)
	binary.BigEndian.PutUint32(buf[10:14], e.PosX)     // PosX (4 bytes)
	binary.BigEndian.PutUint32(buf[14:18], e.PosY)     // PosY (4 bytes)
	binary.BigEndian.PutUint16(buf[18:20], e.Width)    // Width (2 bytes)
	binary.BigEndian.PutUint16(buf[20:22], e.Height)   // Height (2 bytes)
	binary.BigEndian.PutUint16(buf[22:24], e.Pressure) // Pressure (2 bytes)
	binary.BigEndian.PutUint32(buf[24:28], e.Buttons)  // Buttons (4 bytes)
	binary.BigEndian.PutUint32(buf[28:32], e.Buttons)  // Buttons (4 bytes)

	// 3. 一次性发送
	_, err := da.controlConn.Write(buf)
	if err != nil {
		log.Printf("Error sending touch event: %v\n", err)
	}
}

func (da *DataAdapter) SendKeyEvent(e KeyEvent) {
	if da.controlConn == nil {
		return
	}

	buf := make([]byte, 14)

	buf[0] = TYPE_INJECT_KEYCODE                    // Type
	buf[1] = e.Action                               // Action
	binary.BigEndian.PutUint32(buf[2:6], e.KeyCode) // KeyCode (4 bytes)
	binary.BigEndian.PutUint32(buf[6:10], 0)        // Repeat (4 bytes)
	binary.BigEndian.PutUint32(buf[10:14], 0)       // Meta (4 bytes)

	_, err := da.controlConn.Write(buf)
	if err != nil {
		log.Printf("Error sending key event: %v\n", err)
	}
}

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

func (da *DataAdapter) SendScrollEvent(e ScrollEvent) {
	if da.controlConn == nil {
		return
	}
	// Scroll Event Structure (21 bytes):
	// 0: Type (1 byte)
	// 1-4: PosX (4 bytes)
	// 5-8: PosY (4 bytes)
	// 9-10: Width (2 bytes)
	// 11-12: Height (2 bytes)
	// 13-14: HScroll (2 bytes)
	// 15-16: VScroll (2 bytes)
	// 17-20: Buttons (4 bytes)

	buf := make([]byte, 21)
	buf[0] = TYPE_INJECT_SCROLL_EVENT
	binary.BigEndian.PutUint32(buf[1:5], e.PosX)
	binary.BigEndian.PutUint32(buf[5:9], e.PosY)
	binary.BigEndian.PutUint16(buf[9:11], e.Width)
	binary.BigEndian.PutUint16(buf[11:13], e.Height)
	binary.BigEndian.PutUint16(buf[13:15], e.HScroll)
	binary.BigEndian.PutUint16(buf[15:17], e.VScroll)
	binary.BigEndian.PutUint32(buf[17:21], e.Buttons)

	_, err := da.controlConn.Write(buf)
	if err != nil {
		log.Printf("Error sending scroll event: %v\n", err)
	}
}

func (da *DataAdapter) SendUHIDCreateEvent(e UHIDCreateEvent) {
	if da.controlConn == nil {
		return
	}

	nameSize := uint8(len(e.Name)) // 强转为 uint8

	// 包总大小:
	// 1(Type) + 2(ID) + 2(Vendor) + 2(Product) + 1(NameSize) + N(Name) + 2(DescSize) + N(Desc)
	totalSize := 1 + 2 + 2 + 2 + 1 + int(nameSize) + 2 + int(e.ReportDescSize)

	buf := make([]byte, totalSize)
	offset := 0

	// 1. Type
	buf[offset] = e.Type
	offset++

	// 2. ID
	binary.BigEndian.PutUint16(buf[offset:], e.ID)
	offset += 2

	// 3. Vendor
	binary.BigEndian.PutUint16(buf[offset:], e.VendorID)
	offset += 2

	// 4. Product
	binary.BigEndian.PutUint16(buf[offset:], e.ProductID)
	offset += 2

	// 5. Name Size (关键修改：只写 1 个字节)
	buf[offset] = nameSize
	offset++

	// 6. Name Data
	if nameSize > 0 {
		copy(buf[offset:], e.Name)
		offset += int(nameSize)
	}

	// 7. Desc Size (这里依然是 2 字节，因为 Java 里是 parseByteArray(2))
	binary.BigEndian.PutUint16(buf[offset:], e.ReportDescSize)
	offset += 2

	// 8. Desc Data
	copy(buf[offset:], e.ReportDesc)

	log.Printf("Sending UHID_CREATE (Final Fix): ID=%d NameLen=%d", e.ID, nameSize)

	_, err := da.controlConn.Write(buf)
	if err != nil {
		log.Printf("Error sending uhid create event: %v\n", err)
	}
}

func (da *DataAdapter) SendUHIDInputEvent(e UHIDInputEvent) {
	if da.controlConn == nil {
		return
	}
	// Scrcpy UHID Input Protocol:
	// [1] Type
	// [2] ID (uint16)
	// [2] Size (uint16)
	// [N] Data

	totalSize := 1 + 2 + 2 + int(e.Size)
	buf := make([]byte, totalSize)

	offset := 0
	buf[offset] = e.Type
	offset++
	binary.BigEndian.PutUint16(buf[offset:], e.ID)
	offset += 2
	binary.BigEndian.PutUint16(buf[offset:], e.Size)
	offset += 2
	copy(buf[offset:], e.Data)

	_, err := da.controlConn.Write(buf)
	if err != nil {
		log.Printf("Error sending uhid input event: %v\n", err)
	}
}

func (da *DataAdapter) SendUHIDDestroyEvent(e UHIDDestroyEvent) {
	if da.controlConn == nil {
		return
	}
	// Scrcpy UHID Destroy Protocol:
	// [1] Type
	// [2] ID (uint16)

	buf := make([]byte, 3)
	buf[0] = e.Type
	binary.BigEndian.PutUint16(buf[1:], e.ID)

	_, err := da.controlConn.Write(buf)
	if err != nil {
		log.Printf("Error sending uhid destroy event: %v\n", err)
	}
}

func (da *DataAdapter) RequestKeyFrame() error {

	// return nil
	if da.controlConn == nil {
		return nil
	}
	log.Printf("Last Request KeyFrame time: %v Last IDR time: %v", da.lastIDRRequestTime, da.LastIDRTime)
	if time.Since(da.LastIDRTime) < 1*time.Second || time.Since(da.lastIDRRequestTime) < 1*time.Second {
		log.Println("⏳ KeyFrame request too frequent, use cached")
		da.keyFrameMutex.RLock()

		isH265 := da.VideoMeta.CodecID == "h265"
		hasSPS := len(da.LastSPS) > 0
		hasPPS := len(da.LastPPS) > 0
		hasIDR := len(da.LastIDR) > 0
		hasVPS := len(da.LastVPS) > 0

		if !hasSPS || !hasPPS || !hasIDR || (isH265 && !hasVPS) {
			da.keyFrameMutex.RUnlock()
			log.Println("⚠️ Cached keyframe data incomplete, skipping...")
			return nil
		}

		var vpsCopy, spsCopy, ppsCopy []byte
		if isH265 {
			vpsCopy = createCopy(da.LastVPS)
		}
		spsCopy = createCopy(da.LastSPS)
		ppsCopy = createCopy(da.LastPPS)
		// idrCopy = createCopy(da.LastIDR)
		// Check freshness of IDR
		// idrFresh := time.Since(da.LastIDRTime) < 500*time.Millisecond

		da.keyFrameMutex.RUnlock()

		go func() {
			timestamp := time.Now().Unix()
			if isH265 && vpsCopy != nil {
				da.VideoChan <- WebRTCFrame{Data: vpsCopy, Timestamp: timestamp}
			}
			da.VideoChan <- WebRTCFrame{Data: spsCopy, Timestamp: timestamp}
			da.VideoChan <- WebRTCFrame{Data: ppsCopy, Timestamp: timestamp}

			// 为了保证流畅性，即使 IDR 不新鲜也发送
			// if idrCopy != nil {
			// da.VideoChan <- WebRTCFrame{Data: idrCopy, Timestamp: timestamp, NotConfig: true}
			// 	log.Println("✅ Sent cached keyframe data")
			// }
		}()
		return nil
	}
	log.Println("⚡ Sending Request KeyFrame (Type 99)...")
	msg := []byte{ControlMsgTypeReqIDR}
	//<-da.VideoChan
	da.lastIDRRequestTime = time.Now()
	_, err := da.controlConn.Write(msg)
	if err != nil {
		log.Printf("Error sending keyframe request: %v\n", err)
		return err
	}
	return nil
}
