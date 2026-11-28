package consts

var DATA_DIR string

var UDP_RANGE_START uint16
var UDP_RANGE_END uint16
var ICE_SERVERS []string

func GET_DATA_DIR() string {
	return DATA_DIR
}

// func GetDataDir() string {
// 	return DATA_DIR
// }
// func GetWEBRTCUDPRangeStart() uint16 {
// 	return WEBRTC_UDP_RANGE_START
// }
// func GetWEBRTCUDPRangeEnd() uint16 {
// 	return WEBRTC_UDP_RANGE_END
// }
// func GetICEServers() []string {
// 	return ICE_SERVERS
// }

// func SetDataDir(dir string) {
// 	DATA_DIR = dir
// }

// func SetWEBRTCUDPRangeStart(start uint16) {
// 	WEBRTC_UDP_RANGE_START = start
// }

// func SetWEBRTCUDPRangeEnd(end uint16) {
// 	WEBRTC_UDP_RANGE_END = end
// }

// func SetICEServers(servers []string) {
// 	ICE_SERVERS = servers
// }
