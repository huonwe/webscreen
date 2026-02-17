package sagent

// SetSDPBandwidth 在 SDP 的 video m-line 后插入 b=AS:20000 (20Mbps)
// func SetSDPBandwidth(sdp string, bandwidth int) string {
// 	lines := strings.Split(sdp, "\r\n")
// 	var newLines []string
// 	for _, line := range lines {
// 		newLines = append(newLines, line)
// 		if strings.HasPrefix(line, "m=video") {
// 			// b=AS:<bandwidth>  (Application Specific Maximum, 单位 kbps)
// 			// 设置为 20000 kbps = 20 Mbps，远超默认的 2.5 Mbps
// 			newLines = append(newLines, fmt.Sprintf("b=AS:%d", bandwidth))
// 			// 也可以加上 TIAS (Transport Independent Application Specific Maximum, 单位 bps)
// 			// newLines = append(newLines, "b=TIAS:20000000")
// 		}
// 	}
// 	return strings.Join(newLines, "\r\n")
// }
