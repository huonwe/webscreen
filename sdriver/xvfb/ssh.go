package linuxXvfbDriver

import (
	"os"
	"os/exec"
)

// scp and execute Xvfb capturer binary
func PushAndStartXvfb(user, ip, tcpPort, resolution, bitrate, frameRate, codec string) error {
	execCmd := exec.Command("bash", "-c",
		"scp ./capturer_xvfb "+user+"@"+ip+":/tmp/capturer_xvfb && "+
			"ssh "+user+"@"+ip+" 'chmod +x /tmp/capturer_xvfb && "+
			"/tmp/capturer_xvfb -resolution "+resolution+" -tcp_port "+tcpPort+
			" -bitrate "+bitrate+" -framerate "+frameRate+" -codec "+codec+"'")
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Run()
}

func LocalStartXvfb(tcpPort, resolution, bitrate, frameRate, codec string) error {
	execCmd := exec.Command("bash", "-c",
		"chmod +x ./capturer_xvfb && "+
			"./capturer_xvfb -resolution "+resolution+" -tcp_port "+tcpPort+
			" -bitrate "+bitrate+" -framerate "+frameRate+" -codec "+codec)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Start()
}
