package scrcpy

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

func GenerateSCID() string {
	seed := time.Now().UnixNano() + rand.Int63()
	r := rand.New(rand.NewSource(seed))
	return strconv.FormatInt(int64(r.Uint32()&0x7FFFFFFF), 16)
}

func scrcpyParamsToArgs(params map[string]string) []string {
	var args []string
	// keys := []string{
	// 	"scid",
	// 	"max_fps",
	// 	"video",
	// 	"video_codec",
	// 	"video_bit_rate",
	// 	"video_codec_options",
	// 	"video_encoder",
	// 	"audio",
	// 	"audio_bit_rate",
	// 	"audio_codec_options",
	// 	"control",
	// 	"new_display",
	// 	"start_app",
	// 	"max_size",
	// 	"log_level",
	// 	"cleanup",
	// }

	// for _, key := range keys {
	// 	if v, ok := params[key]; ok && v != "" {
	// 		args = append(args, fmt.Sprintf("%s=%s", key, v))
	// 	}
	// }

	for k := range params {
		//如果首字母大写，则跳过
		if len(k) > 0 && k[0] >= 'A' && k[0] <= 'Z' {
			continue
		}

		if v := params[k]; v != "" {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return args
}

func toScrcpyCommand(options map[string]string) string {
	classpath := options["CLASSPATH"]
	version := options["Version"]
	base := fmt.Sprintf("CLASSPATH=%s app_process / com.genymobile.scrcpy.Server %s ",
		classpath, version)
	args := scrcpyParamsToArgs(options)
	return strings.Join(append([]string{base}, args...), " ")
}
