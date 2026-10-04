package agentmode

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/projectbluefin/chairlift/internal/dryrun"
	"github.com/projectbluefin/chairlift/internal/launcher"
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

	origResolve := resolveExecutable
	origStart := startCmd
	defer func() {
		resolveExecutable = origResolve
		startCmd = origStart
	}()

	resolveExecutable = func() string { return "/fake/llmman" }
	called := false
	startCmd = func(cmd *exec.Cmd, rf func(error)) error {
		called = true
		return nil
	}

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

	origResolve := resolveExecutable
	origStart := startCmd
	defer func() {
		resolveExecutable = origResolve
		startCmd = origStart
	}()

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := Launch(ctx, "model", nil)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Launch() with canceled context want context.Canceled, got %v", err)
		}
	})

	t.Run("empty model", func(t *testing.T) {
		err := Launch(context.Background(), "", nil)
		if err == nil || !strings.Contains(err.Error(), "no active model") {
			t.Errorf("Launch() with empty model want error, got %v", err)
		}
	})

	t.Run("missing executable", func(t *testing.T) {
		resolveExecutable = func() string { return "" }
		err := Launch(context.Background(), "unsloth/Qwen3-8B-GGUF:Q4_K_M", nil)
		if err == nil || !strings.Contains(err.Error(), "llmman executable not found") {
			t.Errorf("Launch() with missing executable want error, got %v", err)
		}
	})

	t.Run("start command error", func(t *testing.T) {
		resolveExecutable = func() string { return "/fake/llmman" }
		startFailure := errors.New("cannot spawn goose-desktop")
		startCmd = func(cmd *exec.Cmd, rf func(error)) error {
			return startFailure
		}

		err := Launch(context.Background(), "unsloth/Qwen3-8B-GGUF:Q4_K_M", nil)
		if !errors.Is(err, startFailure) {
			t.Errorf("Launch() error = %v, want %v", err, startFailure)
		}
	})
}

func TestLaunchChildSurvivesCallerContext(t *testing.T) {
	dryrun.Set(false)

	sleepBin, err := exec.LookPath("sleep")
	if err != nil {
		sleepBin = "/bin/sleep"
	}

	origResolve := resolveExecutable
	origStart := startCmd
	defer func() {
		resolveExecutable = origResolve
		startCmd = origStart
	}()

	resolveExecutable = func() string { return sleepBin }

	var capturedCmd *exec.Cmd
	startCmd = func(cmd *exec.Cmd, reportFailure func(error)) error {
		capturedCmd = cmd
		// Adjust args so sleep receives valid numeric duration
		cmd.Args = []string{sleepBin, "2"}
		return launcher.Start(cmd, reportFailure)
	}

	callerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = Launch(callerCtx, "test-model", nil)
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	// Cancel caller's context immediately after Launch returns
	cancel()

	if capturedCmd == nil || capturedCmd.Process == nil {
		t.Fatal("expected process to be started, got nil Process")
	}
	t.Cleanup(func() {
		if capturedCmd != nil && capturedCmd.Process != nil {
			_ = capturedCmd.Process.Kill()
		}
	})

	time.Sleep(50 * time.Millisecond)

	// Send signal 0 to test if child process is still alive and not killed by context cancellation
	if err := capturedCmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Errorf("child process was killed upon caller context cancellation: %v", err)
	}
}
