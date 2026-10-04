package agentmode

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/projectbluefin/chairlift/internal/dryrun"
	"github.com/projectbluefin/chairlift/internal/troubleshoot"
)

func TestReadinessTable(t *testing.T) {
	tests := []struct {
		name      string
		facts     ReadinessFacts
		wantState State
		wantReady bool
	}{
		{
			name: "fresh - no config file",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusMissing,
			},
			wantState: StateExtensionMissing,
			wantReady: false,
		},
		{
			name: "existing - config exists without extension",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusMissing,
			},
			wantState: StateExtensionMissing,
			wantReady: false,
		},
		{
			name: "malformed - config is invalid yaml",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusMalformed,
			},
			wantState: StateExtensionUnsafe,
			wantReady: false,
		},
		{
			name: "unsafe - extension missing toolset fixed or with ssh",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusUnsafe,
			},
			wantState: StateExtensionUnsafe,
			wantReady: false,
		},
		{
			name: "already-correct - extension is valid and verified",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusValid,
			},
			wantState: StateReady,
			wantReady: true,
		},
		{
			name: "daemon unavailable",
			facts: ReadinessFacts{
				DaemonHealthy:   false,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusValid,
			},
			wantState: StateDaemonUnavailable,
			wantReady: false,
		},
		{
			name: "model unavailable",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusValid,
			},
			wantState: StateModelUnavailable,
			wantReady: false,
		},
		{
			name: "packages missing - goose not installed",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  false,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusValid,
			},
			wantState: StatePackagesMissing,
			wantReady: false,
		},
		{
			name: "packages missing - server not installed",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: false,
				ExtensionStatus: troubleshoot.ExtensionStatusValid,
			},
			wantState: StatePackagesMissing,
			wantReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.facts)
			if got != tt.wantState {
				t.Errorf("Evaluate() = %v, want %v", got, tt.wantState)
			}
			if got.Ready() != tt.wantReady {
				t.Errorf("Ready() = %v, want %v", got.Ready(), tt.wantReady)
			}
		})
	}
}

func TestReadinessSubtitlesAndPrerequisites(t *testing.T) {
	states := []State{
		StateReady,
		StateDaemonUnavailable,
		StateModelUnavailable,
		StatePackagesMissing,
		StateExtensionMissing,
		StateExtensionUnsafe,
		StateLaunchFailure,
	}

	for _, state := range states {
		t.Run(state.String(), func(t *testing.T) {
			sub := state.Subtitle("qwen:test")
			if sub == "" {
				t.Errorf("Subtitle() for %v is empty", state)
			}
			prereq := state.MissingPrerequisite()
			if state == StateReady && prereq != "" {
				t.Errorf("MissingPrerequisite() for Ready = %q, want empty", prereq)
			}
			if state != StateReady && prereq == "" {
				t.Errorf("MissingPrerequisite() for %v is empty", state)
			}
		})
	}
}

func TestLaunchDryRun(t *testing.T) {
	dryrun.Set(true)
	defer dryrun.Set(false)

	called := false
	origStart := startCmd
	startCmd = func(cmd *exec.Cmd, rf func(error)) error {
		called = true
		return nil
	}
	defer func() { startCmd = origStart }()

	err := Launch(context.Background(), "unsloth/Qwen3-8B-GGUF:Q4_K_M", nil)
	if err != nil {
		t.Fatalf("Launch() in dry-run returned error: %v", err)
	}
	if called {
		t.Error("Launch() in dry-run executed startCmd")
	}
}

func TestLaunchFailed(t *testing.T) {
	dryrun.Set(false)

	t.Run("empty model", func(t *testing.T) {
		err := Launch(context.Background(), "", nil)
		if err == nil || !strings.Contains(err.Error(), "no active model") {
			t.Errorf("Launch() with empty model want error, got %v", err)
		}
	})

	t.Run("start command error", func(t *testing.T) {
		origStart := startCmd
		startFailure := errors.New("cannot spawn goose-desktop")
		startCmd = func(cmd *exec.Cmd, rf func(error)) error {
			return startFailure
		}
		defer func() { startCmd = origStart }()

		err := Launch(context.Background(), "unsloth/Qwen3-8B-GGUF:Q4_K_M", nil)
		if !errors.Is(err, startFailure) {
			t.Errorf("Launch() error = %v, want %v", err, startFailure)
		}
	})
}
