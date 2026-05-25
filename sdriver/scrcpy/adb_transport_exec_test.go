package scrcpy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"webscreen/sdriver"
)

func TestLocalADBTransportShellRoutesSerial(t *testing.T) {
	argFile := installADBExecHelper(t, "")

	transport, err := NewLocalADBTransport("/fake/adb")
	if err != nil {
		t.Fatalf("NewLocalADBTransport returned error: %v", err)
	}
	if _, err := transport.Shell(context.Background(), "5f9e7947", "echo", "hi"); err != nil {
		t.Fatalf("Shell returned error: %v", err)
	}

	got := readExecHelperArgs(t, argFile)
	want := "/fake/adb\x00-s\x005f9e7947\x00shell\x00echo\x00hi"
	if got != want {
		t.Fatalf("argv mismatch:\ngot  %q\nwant %q", got, want)
	}
}

func TestRemoteADBTransportPrefixesHostPortForCLI(t *testing.T) {
	argFile := installADBExecHelper(t, "5f9e7947 device model:NE2210\n")

	transport, err := NewRemoteADBTransport("mac-coral", "100.85.28.3", 15037, "/fake/adb")
	if err != nil {
		t.Fatalf("NewRemoteADBTransport returned error: %v", err)
	}
	devices, err := transport.Devices(context.Background())
	if err != nil {
		t.Fatalf("Devices returned error: %v", err)
	}
	if len(devices) != 1 || devices[0].Ref.BridgeID != sdriver.BridgeID("mac-coral") {
		t.Fatalf("unexpected devices: %#v", devices)
	}

	got := readExecHelperArgs(t, argFile)
	want := "/fake/adb\x00-H\x00100.85.28.3\x00-P\x0015037\x00devices\x00-l"
	if got != want {
		t.Fatalf("argv mismatch:\ngot  %q\nwant %q", got, want)
	}
}

func TestRemoteADBTransportRequiresExplicitADBExecutable(t *testing.T) {
	_, err := NewRemoteADBTransport("mac-coral", "100.85.28.3", 15037, "")
	if !errors.Is(err, ErrADBExecutableRequired) {
		t.Fatalf("expected ErrADBExecutableRequired, got %v", err)
	}
}

func installADBExecHelper(t *testing.T, stdout string) string {
	t.Helper()

	old := execCommandContext
	argFile := t.TempDir() + string(os.PathSeparator) + "argv.txt"
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		helperArgs := append([]string{"-test.run=TestADBExecHelper", "--", name}, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], helperArgs...)
		cmd.Env = append(os.Environ(),
			"GO_WANT_ADB_EXEC_HELPER=1",
			"ADB_EXEC_HELPER_ARG_FILE="+argFile,
			"ADB_EXEC_HELPER_STDOUT="+stdout,
		)
		return cmd
	}
	t.Cleanup(func() {
		execCommandContext = old
	})
	return argFile
}

func readExecHelperArgs(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read helper args: %v", err)
	}
	return string(data)
}

func TestADBExecHelper(t *testing.T) {
	if os.Getenv("GO_WANT_ADB_EXEC_HELPER") != "1" {
		return
	}
	sep := -1
	for i, arg := range os.Args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep == -1 {
		fmt.Fprint(os.Stderr, "missing --")
		os.Exit(2)
	}
	_ = os.WriteFile(os.Getenv("ADB_EXEC_HELPER_ARG_FILE"), []byte(strings.Join(os.Args[sep+1:], "\x00")), 0600)
	fmt.Print(os.Getenv("ADB_EXEC_HELPER_STDOUT"))
	os.Exit(0)
}
