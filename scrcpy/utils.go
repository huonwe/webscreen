package scrcpy

import (
	"bytes"
	"fmt"
	"io"
)

type SPSInfo struct {
	Width              int     // 视频宽度（像素）
	Height             int     // 视频高度（像素）
	FrameRate          float64 // 估算帧率
	Profile            uint8   // 新增: H.264 Profile
	ConstraintSetFlags uint8
	Level              string // 新增: H.264 Level (字符串表示)
}

// ParseSPS 精确解析SPS关键参数
func ParseSPS(sps []byte, readCroppingFlag bool) (SPSInfo, error) {
	info := SPSInfo{Width: 0, Height: 0}

	// 验证基本SPS格式
	if len(sps) < 8 || (sps[0] != 0x67 && sps[0] != 0x27) {
		return info, fmt.Errorf("无效的SPS数据")
	}

	// 创建位级读取器
	reader := bytes.NewReader(sps)
	bitReader := &BitReader{Reader: reader}

	// 跳过起始字节 (0x00 0x00 0x00 0x01 或 0x00 0x00 0x01)
	startCode := 0
	for i := 0; i < 4; i++ {
		b, err := reader.ReadByte()
		if err != nil {
			return info, err
		}
		startCode = (startCode << 8) | int(b)
	}

	if startCode == 0x00000001 {
		// 标准起始码，已跳过
	} else {
		// 回退
		reader.Seek(0, io.SeekStart)
	}

	// 跳过NAL单元类型
	_, err := reader.ReadByte()
	if err != nil {
		return info, err
	}

	// 1. 读取profile_idc(8位)
	profileIdc, _ := bitReader.ReadUint8(8)
	info.Profile = profileIdc // 可选使用

	// 2. constraint_flags(8位)
	constraintFlags, _ := bitReader.ReadUint8(8)
	info.ConstraintSetFlags = constraintFlags

	// 3. 读取level_idc(8位)
	levelIdc, _ := bitReader.ReadUint8(8)
	info.Level = levelToString(levelIdc)
	// 4. 解析seq_parameter_set_id (ue)
	_, err = bitReader.ReadExpGolomb()
	if err != nil {
		return info, err
	}

	// 5. 处理高级profiles
	if profileIdc == 100 || profileIdc == 110 || profileIdc == 122 ||
		profileIdc == 244 || profileIdc == 44 || profileIdc == 83 ||
		profileIdc == 86 || profileIdc == 118 || profileIdc == 128 {

		// 读取chroma_format_idc (ue)
		chromaFormat, _ := bitReader.ReadExpGolomb()
		if chromaFormat == 3 {
			// 跳过separate_colour_plane_flag (1位)
			bitReader.SkipBits(1)
		}

		// 跳过bit_depth_luma_minus8 (ue)
		bitReader.ReadExpGolomb()

		// 跳过bit_depth_chroma_minus8 (ue)
		bitReader.ReadExpGolomb()

		// 跳过qpprime_y_zero_transform_bypass_flag (1位)
		bitReader.SkipBits(1)

		// 读取seq_scaling_matrix_present_flag (1位)
		scalingPresent, _ := bitReader.ReadUint8(1)
		if scalingPresent == 1 {
			// 跳过scaling_matrix (8次ue)
			for i := 0; i < 8; i++ {
				flag, _ := bitReader.ReadUint8(1)
				if flag == 1 {
					// 跳过scaling_list
					skipScalingList(bitReader)
				}
			}
		}
	}

	// 6. 跳过log2_max_frame_num_minus4 (ue)
	bitReader.ReadExpGolomb()

	// 7. 读取pic_order_cnt_type (ue)
	pocType, _ := bitReader.ReadExpGolomb()
	if pocType == 0 {
		// 跳过log2_max_pic_order_cnt_lsb_minus4 (ue)
		bitReader.ReadExpGolomb()
	} else if pocType == 1 {
		// 跳过delta_pic_order_always_zero_flag (1)
		bitReader.SkipBits(1)
		// 跳过offset_for_non_ref_pic (se)
		bitReader.ReadSignedExpGolomb()
		// 跳过offset_for_top_to_bottom_field (se)
		bitReader.ReadSignedExpGolomb()
		// 读取num_ref_frames_in_pic_order_cnt_cycle (ue)
		count, _ := bitReader.ReadExpGolomb()
		// 跳过offset_for_ref_frame (se × count)
		for i := 0; i < int(count); i++ {
			bitReader.ReadSignedExpGolomb()
		}
	}

	// 8. 跳过num_ref_frames (ue)
	bitReader.ReadExpGolomb()

	// 9. 跳过gaps_in_frame_num_value_allowed_flag (1)
	bitReader.SkipBits(1)

	// 10. 关键: 读取宽度 (pic_width_in_mbs_minus1)
	widthMbsMinus1, _ := bitReader.ReadExpGolomb()
	info.Width = (int(widthMbsMinus1) + 1) * 16

	// 11. 关键: 读取高度 (pic_height_in_map_units_minus1)
	heightMapUnitsMinus1, _ := bitReader.ReadExpGolomb()
	info.Height = (int(heightMapUnitsMinus1) + 1) * 16

	// 12. 处理场模式 (frame_mbs_only_flag)
	frameMbsOnlyFlag, _ := bitReader.ReadUint8(1)
	if frameMbsOnlyFlag == 0 {
		// 场模式，高度加倍
		info.Height *= 2
	}

	// 13. 后续字段 (可选)
	direct8x8Inference, _ := bitReader.ReadUint8(1)
	_ = direct8x8Inference

	// 14. 读取frame_cropping_flag (1位)
	frameCroppingFlag, _ := bitReader.ReadUint8(1)

	// 15. 如果frame_cropping_flag为1，读取裁剪参数
	//是否需要准确的宽高,很耗性能
	if readCroppingFlag {
		if frameCroppingFlag == 1 {
			frameCropLeftOffset, _ := bitReader.ReadExpGolomb()
			frameCropRightOffset, _ := bitReader.ReadExpGolomb()
			frameCropTopOffset, _ := bitReader.ReadExpGolomb()
			frameCropBottomOffset, _ := bitReader.ReadExpGolomb()

			// 应用裁剪参数（假设色度格式为4:2:0）
			subWidthC := int(2)
			subHeightC := int(2)

			// 计算裁剪后的尺寸
			cropLeft := int(frameCropLeftOffset) * subWidthC
			cropRight := int(frameCropRightOffset) * subWidthC
			cropTop := int(frameCropTopOffset) * subHeightC
			cropBottom := int(frameCropBottomOffset) * subHeightC

			info.Width -= cropLeft + cropRight
			info.Height -= cropTop + cropBottom
		}
	}

	// 15. 估算帧率
	info.estimateFrameRate()

	return info, nil
}

type BitReader struct {
	Reader *bytes.Reader
	buffer byte
	bits   uint // 缓冲中剩余的位数
}

func (r *BitReader) ReadBit() (uint8, error) {
	if r.bits == 0 {
		b, err := r.Reader.ReadByte()
		if err != nil {
			return 0, err
		}
		r.buffer = b
		r.bits = 8
	}

	bit := (r.buffer >> 7) & 1
	r.buffer <<= 1
	r.bits--
	return bit, nil
}

func (r *BitReader) SkipBits(n int) error {
	for i := 0; i < n; i++ {
		_, err := r.ReadBit()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *BitReader) ReadUint8(bits uint) (uint8, error) {
	var value uint8
	for i := uint(0); i < bits; i++ {
		bit, err := r.ReadBit()
		if err != nil {
			return 0, err
		}
		value = (value << 1) | bit
	}
	return value, nil
}

func (r *BitReader) ReadUint16(bits uint) (uint16, error) {
	var value uint16
	for i := uint(0); i < bits; i++ {
		bit, err := r.ReadBit()
		if err != nil {
			return 0, err
		}
		value = (value << 1) | uint16(bit)
	}
	return value, nil
}

func (r *BitReader) ReadExpGolomb() (uint32, error) {
	leadingZeros := 0
	for {
		bit, err := r.ReadBit()
		if err != nil {
			return 0, err
		}
		if bit == 1 {
			break
		}
		leadingZeros++
	}

	value := uint32(1 << leadingZeros)
	for i := leadingZeros - 1; i >= 0; i-- {
		bit, err := r.ReadBit()
		if err != nil {
			return 0, err
		}
		if bit == 1 {
			value |= 1 << uint(i)
		}
	}

	return value - 1, nil
}

func (r *BitReader) ReadSignedExpGolomb() (int32, error) {
	value, err := r.ReadExpGolomb()
	if err != nil {
		return 0, err
	}

	if value%2 == 0 {
		return -int32(value/2) - 1, nil
	}
	return int32((value + 1) / 2), nil
}

// 跳过scaling_list
func skipScalingList(br *BitReader) {
	lastScale := 8
	nextScale := 8
	size := 8
	if size > 64 {
		size = 8
	}

	for j := 0; j < size; j++ {
		if nextScale != 0 {
			deltaScale, _ := br.ReadSignedExpGolomb()
			nextScale = (lastScale + int(deltaScale) + 256) % 256
		}
		if nextScale == 0 {
			lastScale = lastScale
		} else {
			lastScale = nextScale
		}
	}
}

// 估算帧率
func (info *SPSInfo) estimateFrameRate() {
	if info.Width == 0 || info.Height == 0 {
		info.FrameRate = 30.0
		return
	}

	megapixels := float64(info.Width*info.Height) / 1000000

	switch {
	case megapixels <= 0.3: // VGA
		info.FrameRate = 30.0
	case megapixels <= 1.0: // 720p
		info.FrameRate = 30.0
	case megapixels <= 2.0: // 1080p
		info.FrameRate = 25.0
	case megapixels <= 8.0: // 4K
		info.FrameRate = 24.0
	default: // 8K+
		info.FrameRate = 24.0
	}
}
func levelToString(level uint8) string {
	major := level / 10
	minor := level % 10
	if minor == 0 {
		return fmt.Sprintf("%d", major)
	}
	return fmt.Sprintf("%d.%d", major, minor)
}
