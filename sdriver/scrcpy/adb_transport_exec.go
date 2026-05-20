package scrcpy

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

var execCommandContext = exec.CommandContext

func runADBCommand(ctx context.Context, adbExecutable string, args ...string) ([]byte, error) {
	if strings.TrimSpace(adbExecutable) == "" {
		return nil, ErrADBExecutableRequired
	}
	cmd := execCommandContext(ctx, adbExecutable, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
