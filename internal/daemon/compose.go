package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	recap "github.com/trnahnh/recap"
	"github.com/trnahnh/recap/internal/config"
)

const composeFileName = "docker-compose.yml"

func writeComposeFile(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating compose dir: %w", err)
	}
	path := filepath.Join(dir, composeFileName)
	if err := os.WriteFile(path, recap.ComposeYAML, 0o644); err != nil {
		return "", fmt.Errorf("writing compose file: %w", err)
	}
	return path, nil
}

func composeCmd(ctx context.Context, cfg *config.Config, composePath string, args ...string) *exec.Cmd {
	full := append([]string{"compose", "-f", composePath}, args...)
	cmd := exec.CommandContext(ctx, "docker", full...)
	cmd.Env = append(os.Environ(), cfg.ComposeEnv()...)
	return cmd
}

func composeUp(ctx context.Context, cfg *config.Config, composePath string) error {
	out, err := composeCmd(ctx, cfg, composePath, "up", "-d").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up failed: %w\n%s", err, out)
	}
	return nil
}

func composeDown(ctx context.Context, cfg *config.Config, composePath string) error {
	out, err := composeCmd(ctx, cfg, composePath, "down").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose down failed: %w\n%s", err, out)
	}
	return nil
}
