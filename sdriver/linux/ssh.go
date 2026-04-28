package linuxDriver

import (
	"os"
	"os/exec"
)

// scp and execute Xvfb capturer binary
func PushAndStartXvfb(user, ip, tcpPort, resolution, bitrate, frameRate, codec string) error {

	execCmd := exec.Command("bash", "-c",
		"scp ./capturer "+user+"@"+ip+":/tmp/capturer && "+
			"ssh "+user+"@"+ip+" 'chmod +x /tmp/capturer && "+
			"/tmp/capturer -resolution "+resolution+" -tcp_port "+tcpPort+
			" -bitrate "+bitrate+" -framerate "+frameRate+" -codec "+codec+"'")

	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Run()
}

func LocalStartXvfb(tcpPort, resolution, bitrate, frameRate, codec string) error {
	execCmd := exec.Command("bash", "-c",
		"chmod +x ./capturer && "+
			"./capturer -resolution "+resolution+" -tcp_port "+tcpPort+
			" -bitrate "+bitrate+" -framerate "+frameRate+" -codec "+codec)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Start()
}
