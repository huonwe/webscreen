//go:build !linux && !android

package scrcpy

import "syscall"

// The on-device (root, no adb) transport only exists on Linux/Android: it relies
// on app_process, /data/local/tmp and unix abstract sockets. On other platforms
// these helpers keep the shared code compiling; the local mode is never enabled
// there (webscreen is started with -local-root only on the device itself).

func localServerGroups() []uint32 { return nil }

func localSysProcAttr(groups []uint32) *syscall.SysProcAttr { return nil }
