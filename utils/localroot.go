package utils

// localRootMode is set when webscreen runs directly on the Android device
// itself (with root) instead of on a host driving the device over adb.
//
// In this mode the scrcpy server is started locally (app_process) and the
// three scrcpy sockets are opened through the local abstract namespace, so
// no adb binary and no adb server are required.
var localRootMode bool

// SetLocalRootMode enables/disables the on-device (root, no adb) mode.
func SetLocalRootMode(v bool) {
	localRootMode = v
}

// IsLocalRootMode reports whether webscreen runs on the device itself.
func IsLocalRootMode() bool {
	return localRootMode
}
