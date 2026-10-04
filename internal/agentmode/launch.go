package agentmode

import (
	"context"
	"errors"
	"log"
	"os/exec"

	"github.com/projectbluefin/chairlift/internal/aistack"
	"github.com/projectbluefin/chairlift/internal/dryrun"
	"github.com/projectbluefin/chairlift/internal/launcher"
)

var startCmd = launcher.Start

// Launch starts Goose Desktop through llmman's invocation-scoped desktop integration
// using the active model. It preserves persistent Goose configuration.
func Launch(ctx context.Context, model string, reportFailure func(error)) error {
	if model == "" {
		return errors.New("no active model selected")
	}
	exe := aistack.Executable()
	if exe == "" {
		return errors.New("llmman executable not found")
	}
	if dryrun.Enabled() {
		log.Printf("[DRY-RUN] would launch Goose Desktop with model %s via llmman", model)
		return nil
	}
	cmd := exec.CommandContext(ctx, exe, "launch", "goose-desktop", "--model", model)
	return startCmd(cmd, reportFailure)
}
