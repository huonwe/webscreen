package linuxDriver

import (
	"log"
	"os"
	"os/exec"
)

// scp and execute recorder binary
func PushAndStartRecorder(user, ip, tcpPort, resolution, bitrate, frameRate, codec, backend string) error {

	execCmd := exec.Command("bash", "-c",
		"scp ./recorder "+user+"@"+ip+":/tmp/recorder && "+
			"ssh "+user+"@"+ip+" 'chmod +x /tmp/recorder && "+
			"/tmp/recorder -resolution "+resolution+" -tcp_port "+tcpPort+
			" -bitrate "+bitrate+" -framerate "+frameRate+" -codec "+codec+" -backend "+backend+"'")

	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Run()
}

func LocalStartRecorder(tcpPort, resolution, bitrate, frameRate, codec, backend string) error {
	// 确保文件有执行权限（建议在部署阶段做好，而不是在代码里每次都 chmod）
	os.Chmod("./recorder", 0755)

	// 直接执行二进制，不要通过 bash -c 拼接字符串
	execCmd := exec.Command("./recorder",
		"-resolution", resolution,
		"-tcp_port", tcpPort,
		"-bitrate", bitrate,
		"-framerate", frameRate,
		"-codec", codec,
		"-backend", backend,
	)

	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	log.Printf("Starting local recorder...")
	return execCmd.Start()
}
