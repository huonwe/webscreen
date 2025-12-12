package adb

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"webcpy/scrcpy"
)

func GenerateSCID() string {
	seed := time.Now().UnixNano() + rand.Int63()
	r := rand.New(rand.NewSource(seed))
	// 生成31位随机整数
	return strconv.FormatInt(int64(r.Uint32()&0x7FFFFFFF), 16)
}

// 将ScrcpyParams转为 key=value 格式的参数列表
func ScrcpyParamsToArgs(p scrcpy.ScrcpyParams) []string {
	args := []string{
		// fmt.Sprintf("max_size=%s", p.MaxSize),
		fmt.Sprintf("max_fps=%s", p.MaxFPS),
		fmt.Sprintf("video_bit_rate=%s", p.VideoBitRate),
		fmt.Sprintf("control=%s", p.Control),
		fmt.Sprintf("audio=%s", p.Audio),
		fmt.Sprintf("video_codec=%s", p.VideoCodec),
	}
	if p.MaxSize != "" {
		args = append(args, fmt.Sprintf("max_size=%s", p.MaxSize))
	}
	if p.VideoCodecOptions != "" {
		args = append(args, fmt.Sprintf("video_codec_options=%s", p.VideoCodecOptions))
	}
	args = append(args, fmt.Sprintf("log_level=%s", p.LogLevel))
	return args
}

func GetScrcpyCommand(params scrcpy.ScrcpyParams) string {
	base := fmt.Sprintf("CLASSPATH=%s app_process / com.genymobile.scrcpy.Server %s ",
		params.CLASSPATH, params.Version)
	args := ScrcpyParamsToArgs(params)
	return strings.Join(append([]string{base}, args...), " ")
}
