package agentmode

import (
	"testing"

	"github.com/projectbluefin/chairlift/internal/troubleshoot"
)

func TestDispatchAskBluefin(t *testing.T) {
	tests := []struct {
		name       string
		facts      ReadinessFacts
		wantAction DispatchAction
		wantModel  string
		wantState  State
		wantReason string
	}{
		{
			name: "all ready -> dispatch launch",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusValid,
			},
			wantAction: DispatchLaunch,
			wantModel:  "unsloth/Qwen3-8B-GGUF:Q4_K_M",
			wantState:  StateReady,
			wantReason: "",
		},
		{
			name: "daemon unavailable -> present agents",
			facts: ReadinessFacts{
				DaemonHealthy:   false,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusValid,
			},
			wantAction: DispatchPresentAgents,
			wantModel:  "",
			wantState:  StateDaemonUnavailable,
			wantReason: "Agent Mode is not running.",
		},
		{
			name: "model unavailable -> present agents",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusValid,
			},
			wantAction: DispatchPresentAgents,
			wantModel:  "",
			wantState:  StateModelUnavailable,
			wantReason: "No model is selected in Agent Mode.",
		},
		{
			name: "goose missing -> present agents",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  false,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusValid,
			},
			wantAction: DispatchPresentAgents,
			wantModel:  "",
			wantState:  StatePackagesMissing,
			wantReason: "Goose Desktop or linux-mcp-server is not installed.",
		},
		{
			name: "linux-mcp-server missing -> present agents",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: false,
				ExtensionStatus: troubleshoot.ExtensionStatusValid,
			},
			wantAction: DispatchPresentAgents,
			wantModel:  "",
			wantState:  StatePackagesMissing,
			wantReason: "Goose Desktop or linux-mcp-server is not installed.",
		},
		{
			name: "extension missing -> present agents",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusMissing,
			},
			wantAction: DispatchPresentAgents,
			wantModel:  "",
			wantState:  StateExtensionMissing,
			wantReason: "Linux tools extension in Goose is not configured.",
		},
		{
			name: "extension unsafe -> present agents",
			facts: ReadinessFacts{
				DaemonHealthy:   true,
				ActiveModel:     "unsloth/Qwen3-8B-GGUF:Q4_K_M",
				GooseInstalled:  true,
				ServerInstalled: true,
				ExtensionStatus: troubleshoot.ExtensionStatusUnsafe,
			},
			wantAction: DispatchPresentAgents,
			wantModel:  "",
			wantState:  StateExtensionUnsafe,
			wantReason: "Linux tools extension in Goose is unsafe.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := Dispatch(tt.facts)
			if decision.Action != tt.wantAction {
				t.Errorf("Dispatch().Action = %v, want %v", decision.Action, tt.wantAction)
			}
			if decision.Model != tt.wantModel {
				t.Errorf("Dispatch().Model = %q, want %q", decision.Model, tt.wantModel)
			}
			if decision.State != tt.wantState {
				t.Errorf("Dispatch().State = %v, want %v", decision.State, tt.wantState)
			}
			if decision.Reason != tt.wantReason {
				t.Errorf("Dispatch().Reason = %q, want %q", decision.Reason, tt.wantReason)
			}
		})
	}
}
