package actionstate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFeaturesPageDeveloperModeUsesGate(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate wiring_test.go")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))

	viewsPath := filepath.Join(repoRoot, "internal", "views", "views.go")
	viewsSource, err := os.ReadFile(viewsPath)
	if err != nil {
		t.Fatalf("read %s: %v", viewsPath, err)
	}
	viewsText := string(viewsSource)
	if !strings.Contains(viewsText, "developerGate") || !strings.Contains(viewsText, "actionstate.Gate") {
		t.Errorf("views.go UserHome does not contain developerGate actionstate.Gate field")
	}

	featuresPath := filepath.Join(repoRoot, "internal", "views", "features_page.go")
	featuresSource, err := os.ReadFile(featuresPath)
	if err != nil {
		t.Fatalf("read %s: %v", featuresPath, err)
	}
	featuresText := string(featuresSource)

	for _, required := range []string{
		`uh.developerGate.TryStart()`,
		`uh.developerGate.Reset()`,
		// The recursion guard used to be a bare `toggle.SetState(` beside
		// every `SetActive`, which only held while every call site
		// remembered to pair them. guardedSwitch owns that pairing —
		// newGuardedSwitch sets both, and set() re-enters behind an
		// `applying` flag the handler checks — so the property is now
		// asserted where it is enforced rather than at each call site.
		`toggle.set(`,
		`newGuardedSwitch(`,
	} {
		if !strings.Contains(featuresText, required) {
			t.Errorf("features_page wiring does not contain %q", required)
		}
	}
}

// A button that is made sensitive again after a run must release its gate;
// Complete permanently rejects every future click, including retries after
// a failed or dry-run action. Views cannot be imported by headless tests.
func TestRepeatableControlsReleaseTheirGates(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate wiring_test.go")
	}
	viewsDir := filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
	for file, gates := range map[string][]string{
		"updates_page.go": {"driverGate"},
		"reset.go":        {"powerwashGate", "factoryResetGate"},
		"versions.go":     {"pinGate"},
		"recovery.go":     {"unpinGate"},
		"agents_page.go":  {"agentPresetGate"},
		// Set Up and Launch share one gate, and both are offered again.
		"troubleshoot.go": {"gooseGate", "askBluefinGate"},
		// The developer switch and the optional feed setup behind it are
		// both repeatable: the switch is used again after every toggle, and
		// the setup gate has to reopen when its worker finishes or a second
		// enable could never install anything.
		"features_page.go": {"developerGate", "developerFeedGate"},
	} {
		data, err := os.ReadFile(filepath.Join(viewsDir, file))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, gate := range gates {
			if !strings.Contains(text, "uh."+gate+".TryStart()") || !strings.Contains(text, "uh."+gate+".Reset()") || strings.Contains(text, "uh."+gate+".Complete()") {
				t.Errorf("%s: repeatable %s must start and reset, never complete", file, gate)
			}
		}
	}
}

// bootc rollback toggles the selected deployment. A successful live click
// must close its gate; only a failed attempt or dry-run preview may retry.
func TestRollbackGateCompletesOnlyAfterLiveSuccess(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate wiring_test.go")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "recovery.go"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(data), "func (uh *UserHome) onBootcRollbackClicked()", 2)
	if len(parts) != 2 {
		t.Fatal("rollback handler not found")
	}
	body := parts[1]
	for _, required := range []string{"uh.bootcRollbackGate.TryStart()", "if decision.Confirm {", "uh.bootcRollbackGate.Complete()", "button.SetSensitive(false)", "uh.bootcRollbackGate.Reset()", "button.SetSensitive(true)"} {
		if !strings.Contains(body, required) {
			t.Errorf("rollback must remain one-shot on live success but retry after failure or preview: missing %q", required)
		}
	}
}

// Panel toggle mirrors the persisted SavedPanelIcon/SavedPanelMode into the
// page's in-memory view of state. Under --dry-run the writes are no-ops, so
// the mirror has to be skipped too — otherwise the next enable sees
// non-empty saved values in memory and skips the capture the user asked
// for. Issue #422.
//
// Both the enable (capture) and the disable (reset) path need their own
// gate, so the assertions below match each gate together with the
// assignments it has to contain. Matching the gate line on its own would
// still pass if one of the two gates were dropped, or if an assignment were
// moved out from under its gate.
func TestLiveryPanelToggleDoesNotMutateInMemoryStateUnderDryRun(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate wiring_test.go")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "livery_actions.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		`actionmsg.LiveryToggle(dryrun.Enabled(), enabled, pageview.LiverySectionName(surface))`,
		`if enabled && surface == livery.Panel && savedIcon == "" && savedMode == "" {`,
		`if err := livery.SetString(ctx, livery.KeySavedPanelIcon, icon); err != nil {`,
		`if err := livery.SetString(ctx, livery.KeySavedPanelMode, mode); err != nil {`,
		`if err := livery.ClearPanelSettings(ctx, savedIcon, savedMode); err != nil {`,
	} {
		if !strings.Contains(text, required) {
			t.Errorf("panel capture/revert omits %q", required)
		}
	}
	for name, values := range map[string][2]string{
		"capture": {"icon", "mode"},
		"reset":   {`""`, `""`},
	} {
		snippet := "\t\t\tif decision.MutateUI {\n" +
			"\t\t\t\tsgtk.RunOnMainThread(func() {\n" +
			"\t\t\t\t\tuh.liveryState.SavedPanelIcon = " + values[0] + "\n" +
			"\t\t\t\t\tuh.liveryState.SavedPanelMode = " + values[1] + "\n" +
			"\t\t\t\t})\n\t\t\t}\n"
		if !strings.Contains(text, snippet) {
			t.Errorf("panel %s mirror bypasses its dry-run mutation decision", name)
		}
	}
}

// When the asynchronous enable of the Top Bar or Files mark lands, the
// "Rotate at Login" switch's sensitivity must be re-evaluated using the
// same formula applyLiveryState uses on load —
// pageview.LiveryRotationAvailable(panelAvailable && state.PanelEnabled, id).
// finishLiveryToggle runs that re-evaluation through
// setLiverySectionSensitive and syncLiveryRotateSensitive, so the test
// pins both: the panel branch must combine the caller's `enabled` flag
// with liveryPanelAvailable (issue #496), and the toggle-completion
// handler must call the section-sensitive re-evaluation under
// `if outcome.Commit` so a failed or previewed toggle does not pretend
// the section became available.
func TestLiveryPanelToggleRefreshesRotateSensitivityOnCommit(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate wiring_test.go")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "livery_actions.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	// setLiverySectionSensitive's Panel case must gate every visible-state
	// advance on liveryPanelAvailable so the toggle-completion handler
	// computes the same sensitivity the load pass did. Matching the body
	// in one contiguous block rules out a regression that re-inlines the
	// unguarded `enabled` argument the issue calls out.
	sectionSensitivePanel := "case livery.Panel:\n" +
		"\t\tpanelAvailable := uh.liveryPanelAvailable && enabled\n" +
		"\t\tif uh.liveryPanelMarkRow != nil {\n" +
		"\t\t\tuh.liveryPanelMarkRow.SetSensitive(panelAvailable)\n" +
		"\t\t}\n" +
		"\t\tif uh.liveryFoundationGrid != nil {\n" +
		"\t\t\tuh.liveryFoundationGrid.SetSensitive(panelAvailable)\n" +
		"\t\t}\n" +
		"\t\tuh.syncLiveryRotateSensitive(s, panelAvailable)"
	if !strings.Contains(text, sectionSensitivePanel) {
		t.Errorf("setLiverySectionSensitive(panel) does not gate sensitivity on liveryPanelAvailable (issue #496)")
	}

	// finishLiveryToggle must re-evaluate the section's sub-rows under
	// `if outcome.Commit`, so a successful toggle refreshes the rotate row
	// on the main thread and a failed or previewed one reverts the master
	// switch without changing sub-row sensitivity. The two halves are
	// matched as one block so a regression that drops the re-evaluation
	// or moves it out from under the gate fails the test.
	finishToggle := "\t\tif outcome.Commit {\n" +
		"\t\t\tuh.setLiveryToggleState(s, enabled)\n" +
		"\t\t\tuh.setLiverySectionSensitive(s, enabled)\n" +
		"\t\t} else if toggle != nil {"
	if !strings.Contains(text, finishToggle) {
		t.Errorf("finishLiveryToggle does not re-evaluate section sensitivity only on commit (issue #496)")
	}

	// applyLiveryState must keep computing the rotate sensitivity with
	// the same `panelAvailable && state.PanelEnabled` formula, so the
	// toggle-completion handler refreshes to the same value the load
	// pass set. The wiring test pins the source line so the two
	// computations cannot drift again.
	liveryPath := filepath.Join(filepath.Dir(filename), "..", "livery_page.go")
	liverySource, err := os.ReadFile(liveryPath)
	if err != nil {
		t.Fatal(err)
	}
	liveryText := string(liverySource)
	rotateLine := "\tif uh.liveryPanelRotate != nil {\n" +
		"\t\tuh.liveryPanelRotate.SetSensitive(\n" +
		"\t\t\tpageview.LiveryRotationAvailable(panelAvailable && state.PanelEnabled, uh.liveryState.PanelID))"
	if !strings.Contains(liveryText, rotateLine) {
		t.Errorf("applyLiveryState rotate sensitivity diverged from the toggle-completion formula")
	}
}
