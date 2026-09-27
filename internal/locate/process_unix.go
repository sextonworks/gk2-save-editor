//go:build !windows

package locate

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

func GameRunning(ctx context.Context) (bool, error) {
	err := exec.CommandContext(ctx, "pgrep", "-qf", ExeName).Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("pgrep: %w", err)
}
