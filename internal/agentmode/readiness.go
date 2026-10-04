// Package agentmode provides readiness evaluation, launch coordination, and
// the Ask Bluefin dispatcher for Agent Mode's Goose Desktop client.
package agentmode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/projectbluefin/chairlift/internal/aistack"
	"github.com/projectbluefin/chairlift/internal/homebrew"
	"github.com/projectbluefin/chairlift/internal/troubleshoot"
)

// State represents the readiness of Goose Desktop as the Agent Mode client.
type State int

const (
	// StateReady indicates all prerequisites are satisfied: healthy daemon,
	// active model, installed packages, and verified Linux MCP extension.
	StateReady State = iota
	// StateDaemonUnavailable indicates llmman daemon is not running or unhealthy.
	StateDaemonUnavailable
	// StateModelUnavailable indicates no active model is selected.
	StateModelUnavailable
	// StatePackagesMissing indicates Goose Desktop or linux-mcp-server is missing.
	StatePackagesMissing
	// StateExtensionMissing indicates Goose config or linux-tools extension is missing.
	StateExtensionMissing
	// StateExtensionUnsafe indicates linux-tools extension is malformed or violates safety policy.
	StateExtensionUnsafe
	// StateLaunchFailure indicates the process failed to launch.
	StateLaunchFailure
)

// String returns a human-readable representation of State.
func (s State) String() string {
	switch s {
	case StateReady:
		return "Ready"
	case StateDaemonUnavailable:
		return "DaemonUnavailable"
	case StateModelUnavailable:
		return "ModelUnavailable"
	case StatePackagesMissing:
		return "PackagesMissing"
	case StateExtensionMissing:
		return "ExtensionMissing"
	case StateExtensionUnsafe:
		return "ExtensionUnsafe"
	case StateLaunchFailure:
		return "LaunchFailure"
	default:
		return fmt.Sprintf("State(%d)", int(s))
	}
}

// Ready reports whether all launch prerequisites are satisfied.
func (s State) Ready() bool {
	return s == StateReady
}

// MissingPrerequisite returns the description of the unmet requirement.
func (s State) MissingPrerequisite() string {
	switch s {
	case StateDaemonUnavailable:
		return "Agent Mode is not running."
	case StateModelUnavailable:
		return "No model is selected in Agent Mode."
	case StatePackagesMissing:
		return "Goose Desktop is not installed."
	case StateExtensionMissing:
		return "Linux tools extension in Goose is not configured."
	case StateExtensionUnsafe:
		return "Linux tools extension in Goose is unsafe."
	case StateLaunchFailure:
		return "Failed to launch Goose Desktop."
	default:
		return ""
	}
}

// Subtitle returns a user-facing explanation of the current state.
func (s State) Subtitle(model string) string {
	switch s {
	case StateReady:
		if model != "" {
			return fmt.Sprintf("Ready to launch with %s.", model)
		}
		return "Ready to launch."
	case StateDaemonUnavailable:
		return "Turn on Agent Mode to launch Goose."
	case StateModelUnavailable:
		return "Choose a model to launch Goose."
	case StatePackagesMissing:
		return "Goose Desktop or linux-mcp-server is not installed."
	case StateExtensionMissing:
		return "Linux tools extension in Goose is not configured."
	case StateExtensionUnsafe:
		return "Linux tools extension in Goose has unsafe settings."
	case StateLaunchFailure:
		return "Failed to launch Goose Desktop."
	default:
		return ""
	}
}

// ReadinessFacts captures observed facts about Agent Mode and Goose.
type ReadinessFacts struct {
	DaemonHealthy   bool
	ActiveModel     string
	GooseInstalled  bool
	ServerInstalled bool
	ExtensionStatus troubleshoot.ExtensionStatus
	DryRun          bool
}

// Evaluate determines the readiness State from the supplied facts.
func Evaluate(facts ReadinessFacts) State {
	if !facts.DaemonHealthy {
		return StateDaemonUnavailable
	}
	if facts.ActiveModel == "" {
		return StateModelUnavailable
	}
	if !facts.GooseInstalled || !facts.ServerInstalled {
		return StatePackagesMissing
	}
	switch facts.ExtensionStatus {
	case troubleshoot.ExtensionStatusMissing:
		return StateExtensionMissing
	case troubleshoot.ExtensionStatusMalformed, troubleshoot.ExtensionStatusUnsafe:
		return StateExtensionUnsafe
	case troubleshoot.ExtensionStatusValid:
		return StateReady
	default:
		return StateExtensionUnsafe
	}
}

// Injection seams for testing live observation.
var (
	lookTool       = defaultLookTool
	checkExtension = troubleshoot.VerifyExtensionOnDisk
	checkDaemon    = aistack.Healthy
	checkModel     = aistack.ReadActiveModel
)

func defaultLookTool(name string) bool {
	if path, err := exec.LookPath(name); err == nil && path != "" {
		return true
	}
	brew := homebrew.ExecutablePath()
	if brew == "" {
		return false
	}
	path, err := exec.LookPath(filepath.Join(filepath.Dir(brew), name))
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// ObserveLive queries the live system state and evaluates readiness.
func ObserveLive(ctx context.Context) (State, ReadinessFacts, error) {
	daemonHealthy := checkDaemon(ctx)
	var activeModel string
	if daemonHealthy {
		model, err := checkModel(ctx)
		if err == nil {
			activeModel = model
		}
	}
	gooseInstalled := lookTool("goose-desktop")
	serverInstalled := lookTool("linux-mcp-server")
	extStatus := checkExtension()

	facts := ReadinessFacts{
		DaemonHealthy:   daemonHealthy,
		ActiveModel:     activeModel,
		GooseInstalled:  gooseInstalled,
		ServerInstalled: serverInstalled,
		ExtensionStatus: extStatus,
	}
	return Evaluate(facts), facts, nil
}
