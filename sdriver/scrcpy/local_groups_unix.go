//go:build linux || android

package scrcpy

import (
	"os"
	"syscall"
)

// appendUniqueGroup appends g unless it is already present.
func appendUniqueGroup(groups []uint32, g uint32) []uint32 {
	for _, existing := range groups {
		if existing == g {
			return groups
		}
	}
	return append(groups, g)
}

// localServerGroups returns the supplementary groups the locally started
// scrcpy server must keep.
//
// scrcpy drops privileges to the shell uid as soon as it starts, but it keeps
// this process' supplementary groups, and /dev/uhid is 0660 uhid:uhid. Without
// the uhid group, virtual mouse/keyboard/gamepad input fails with
//
//	open failed: EACCES (Permission denied)   (UhidManager.open)
//
// and the controller thread then kills the whole scrcpy server, which freezes
// the stream.
func localServerGroups() []uint32 {
	groups := []uint32{0}
	if current, err := os.Getgroups(); err == nil {
		for _, g := range current {
			groups = appendUniqueGroup(groups, uint32(g))
		}
	}
	gid := uint32(AID_UHID)
	if fi, err := os.Stat(UHID_DEVICE); err == nil {
		if st, ok := fi.Sys().(*syscall.Stat_t); ok && st.Gid != 0 {
			gid = st.Gid
		}
	}
	return appendUniqueGroup(groups, gid)
}

// localSysProcAttr keeps uid/gid 0 and the supplementary groups gathered above.
func localSysProcAttr(groups []uint32) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:    0,
			Gid:    0,
			Groups: groups,
		},
	}
}
