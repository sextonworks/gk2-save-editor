//go:build windows

package locate

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func GameRunning(ctx context.Context) (bool, error) {
	out, err := exec.CommandContext(ctx, "tasklist", "/FI", "IMAGENAME eq "+ExeName, "/NH").Output()
	if err != nil {
		return false, fmt.Errorf("tasklist: %w", err)
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(ExeName)), nil
}
