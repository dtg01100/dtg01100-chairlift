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

var (
	resolveExecutable = aistack.Executable
	startCmd          = launcher.Start
)

// Launch starts Goose Desktop through llmman's invocation-scoped desktop integration
// using the active model. It uses exec.Command without a context so the long-lived GUI
// process is not tied to a caller's timeout or cancellation. Persistent Goose
// configuration is left unchanged.
func Launch(ctx context.Context, model string, reportFailure func(error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if model == "" {
		return errors.New("no active model selected")
	}
	exe := resolveExecutable()
	if exe == "" {
		return errors.New("llmman executable not found")
	}
	if dryrun.Enabled() {
		log.Printf("[DRY-RUN] would launch Goose Desktop with model %s via llmman", model)
		return nil
	}
	cmd := exec.Command(exe, "launch", "goose-desktop", "--model", model)
	return startCmd(cmd, reportFailure)
}
