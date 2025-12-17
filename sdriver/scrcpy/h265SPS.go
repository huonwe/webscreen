package scrcpy

import (
	"bytes"
	"fmt"
)

// ParseSPS_H265 解析 H.265 SPS NAL Unit (不包含 Start Code 00000001)
func ParseSPS_H265(sps []byte) (SPSInfo, error) {
	info := SPSInfo{}

	if len(sps) < 2 {
		return info, fmt.Errorf("SPS data too short")
	}

	// 1. 去除防竞争码 (Emulation Prevention Bytes: 00 00 03 -> 00 00)
	rbsp := RemoveEmulationPreventionBytes(sps)

	// 2. 初始化 BitReader
	br := &BitReader{Reader: bytes.NewReader(rbsp)}

	// 3. 解析 NAL Header (2 bytes)
	// H.265 NAL Header: F(1) + Type(6) + LayerId(6) + TID(3)
	br.ReadBits(1)               // forbidden_zero_bit
	nalType, _ := br.ReadBits(6) // nal_unit_type
	br.ReadBits(6)               // nuh_layer_id
	br.ReadBits(3)               // nuh_temporal_id_plus1

	// SPS NAL Type 应该是 33 (0x21)
	if nalType != 33 {
		return info, fmt.Errorf("not an H.265 SPS NAL unit (type: %d)", nalType)
	}

	// 4. 解析 SPS Body
	// sps_video_parameter_set_id: u(4)
	br.ReadBits(4)

	// sps_max_sub_layers_minus1: u(3)
	maxSubLayersMinus1, _ := br.ReadBits(3)

	// sps_temporal_id_nesting_flag: u(1)
	br.ReadBits(1)

	// --- Profile Tier Level (PTL) ---
	// 这部分非常关键，必须正确跳过 sub-layers 才能读到后面的尺寸信息
	profile, tier, level, err := parseProfileTierLevel(br, maxSubLayersMinus1)
	if err != nil {
		return info, err
	}
	info.Profile = profile
	info.Level = fmt.Sprintf("%.1f", float32(level)/30.0)
	if tier == 1 {
		info.Tier = "High"
	} else {
		info.Tier = "Main"
	}

	// sps_seq_parameter_set_id: ue(v)
	br.ReadExpGolomb()

	// chroma_format_idc: ue(v)
	chromaFormatIDC, _ := br.ReadExpGolomb()
	info.ChromaFormat = chromaFormatIDC
	if chromaFormatIDC == 3 {
		br.ReadBits(1) // separate_colour_plane_flag
	}

	// pic_width_in_luma_samples: ue(v)
	width, _ := br.ReadExpGolomb()

	// pic_height_in_luma_samples: ue(v)
	height, _ := br.ReadExpGolomb()

	// conformance_window_flag: u(1) (用于裁剪，例如 1920x1088 -> 1920x1080)
	conformanceWindowFlag, _ := br.ReadBits(1)

	confWinLeftOffset := uint32(0)
	confWinRightOffset := uint32(0)
	confWinTopOffset := uint32(0)
	confWinBottomOffset := uint32(0)

	if conformanceWindowFlag == 1 {
		confWinLeftOffset, _ = br.ReadExpGolomb()
		confWinRightOffset, _ = br.ReadExpGolomb()
		confWinTopOffset, _ = br.ReadExpGolomb()
		confWinBottomOffset, _ = br.ReadExpGolomb()
	}

	// 计算实际显示分辨率
	// H.265 中裁剪单位取决于 Chroma Format
	// Chroma Format IDC: 0=Monochrome, 1=4:2:0, 2=4:2:2, 3=4:4:4
	subWidthC := uint32(1)
	subHeightC := uint32(1)

	if chromaFormatIDC == 1 { // 4:2:0 (最常见)
		subWidthC = 2
		subHeightC = 2
	} else if chromaFormatIDC == 2 { // 4:2:2
		subWidthC = 2
		subHeightC = 1
	}
	// 4:4:4 (3) 都是 1

	info.Width = width - (confWinLeftOffset+confWinRightOffset)*subWidthC
	info.Height = height - (confWinTopOffset+confWinBottomOffset)*subHeightC

	return info, nil
}

// parseProfileTierLevel 解析 PTL 结构，这对跳过比特位至关重要
func parseProfileTierLevel(br *BitReader, maxSubLayersMinus1 uint32) (uint8, uint8, uint8, error) {
	// general_profile_space: u(2)
	br.ReadBits(2)
	// general_tier_flag: u(1)
	tierFlag, _ := br.ReadBits(1)
	// general_profile_idc: u(5)
	profileIDC, _ := br.ReadBits(5)

	// general_profile_compatibility_flag: u(32)
	br.ReadBits(32)

	// general_constraint_indicator_flags: u(48)
	br.ReadBits(32)
	br.ReadBits(16)

	// general_level_idc: u(8)
	levelIDC, _ := br.ReadBits(8)

	// 处理 sub_layers 的存在标志
	subLayerProfilePresentFlag := make([]bool, maxSubLayersMinus1)
	subLayerLevelPresentFlag := make([]bool, maxSubLayersMinus1)

	for i := uint32(0); i < maxSubLayersMinus1; i++ {
		val, _ := br.ReadBits(1)
		subLayerProfilePresentFlag[i] = val == 1
		val, _ = br.ReadBits(1)
		subLayerLevelPresentFlag[i] = val == 1
	}

	if maxSubLayersMinus1 > 0 {
		for i := uint32(maxSubLayersMinus1); i < 8; i++ {
			br.ReadBits(2) // reserved_zero_2bits
		}
	}

	// 跳过 sub_layers 的数据
	for i := uint32(0); i < maxSubLayersMinus1; i++ {
		if subLayerProfilePresentFlag[i] {
			br.ReadBits(2)  // sub_layer_profile_space
			br.ReadBits(1)  // sub_layer_tier_flag
			br.ReadBits(5)  // sub_layer_profile_idc
			br.ReadBits(32) // sub_layer_profile_compatibility_flag
			br.ReadBits(32) // sub_layer_constraint_indicator_flags (part 1)
			br.ReadBits(16) // sub_layer_constraint_indicator_flags (part 2)
		}
		if subLayerLevelPresentFlag[i] {
			br.ReadBits(8) // sub_layer_level_idc
		}
	}

	return uint8(profileIDC), uint8(tierFlag), uint8(levelIDC), nil
}
