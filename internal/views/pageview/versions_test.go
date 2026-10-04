package pageview

import (
	"strings"
	"testing"
	"time"

	"github.com/projectbluefin/chairlift/internal/registrytags"
)

func TestCatalogStreamFollowsTheStreamOfAPinnedBuild(t *testing.T) {
	tests := []struct{ tag, want string }{
		{"stable", "stable"},
		{"lts-testing", "lts-testing"},
		{"stable-20260908", "stable"},
		{"lts-testing.20260621", "lts-testing"},
		{"", ""},
	}
	for _, test := range tests {
		if got := CatalogStream(test.tag); got != test.want {
			t.Errorf("CatalogStream(%q) = %q, want %q", test.tag, got, test.want)
		}
	}
}

// Tags as GHCR lists them for ghcr.io/ublue-os/bluefin-dx on 2026-09-24:
// one build per day is reachable under several streams' spellings.
func TestPublishedVersionsListsOneRowPerDayOfTheRunningStream(t *testing.T) {
	builds := registrytags.Builds([]string{
		"stable", "latest", "sha256-0f3c.sig",
		"gts-20260922", "stable-20260922", "stable-daily-20260922", "stable-44.20260922",
		"gts-20260915", "stable-20260915", "stable-daily-20260915",
		"latest-20260908", "stable-20260908", "stable-daily-20260908",
		"stable-20260901",
	}, time.Time{})

	got := PublishedVersions(builds, "stable", "44.20260908", "44.20260901")
	want := []PublishedVersion{
		{Row: Row{Title: "22 September 2026", Subtitle: "Published as stable-20260922"}, Day: "20260922"},
		{Row: Row{Title: "15 September 2026", Subtitle: "Published as stable-20260915"}, Day: "20260915"},
		{Row: Row{Title: "8 September 2026", Subtitle: "Running now · Published as stable-20260908"}, Day: "20260908"},
		{Row: Row{Title: "1 September 2026", Subtitle: "Your previous version · Published as stable-20260901"}, Day: "20260901"},
	}
	if len(got) != len(want) {
		t.Fatalf("PublishedVersions() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("PublishedVersions()[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestPublishedVersionsMarksNothingForAnUnreadableVersion(t *testing.T) {
	builds := registrytags.Builds([]string{"stable-20260908"}, time.Time{})
	got := PublishedVersions(builds, "stable", "", "not-a-build")
	if len(got) != 1 || got[0].Subtitle != "Published as stable-20260908" || got[0].Day != "20260908" {
		t.Errorf("PublishedVersions() = %#v, want one unmarked row", got)
	}
}

func TestPinConfirmationStatesTheTargetAndRestart(t *testing.T) {
	title, body := PinConfirmation("20 September 2026")
	if title != "Pin to 20 September 2026?" {
		t.Errorf("PinConfirmation() title = %q, want %q", title, "Pin to 20 September 2026?")
	}
	for _, fragment := range []string{"20 September 2026", "restart", "return to the stream"} {
		if !strings.Contains(body, fragment) {
			t.Errorf("PinConfirmation() body = %q, want it to contain %q", body, fragment)
		}
	}
}

func TestUnpinConfirmationStatesStreamAndRestart(t *testing.T) {
	title, body := UnpinConfirmation("latest")
	if title != "Return to Stream?" {
		t.Errorf("UnpinConfirmation() title = %q, want %q", title, "Return to Stream?")
	}
	for _, fragment := range []string{"latest", "restart", "regular updates"} {
		if !strings.Contains(body, fragment) {
			t.Errorf("UnpinConfirmation() body = %q, want it to contain %q", body, fragment)
		}
	}
}

func TestUnpinRowReflectsSupport(t *testing.T) {
	supported := UnpinRow("latest", true)
	if supported.Title != "Return to stream" {
		t.Errorf("UnpinRow(supported).Title = %q, want %q", supported.Title, "Return to stream")
	}
	if !strings.Contains(supported.Subtitle, "latest") {
		t.Errorf("UnpinRow(supported).Subtitle = %q, want it to name the stream", supported.Subtitle)
	}

	unsupported := UnpinRow("latest", false)
	if !strings.Contains(unsupported.Subtitle, "not supported") {
		t.Errorf("UnpinRow(unsupported).Subtitle = %q, want it to explain lack of support", unsupported.Subtitle)
	}
}

func TestPinAndUnpinUnsupportedExplanations(t *testing.T) {
	if got := PinUnsupportedExplanation(); !strings.Contains(got, "not supported") {
		t.Errorf("PinUnsupportedExplanation() = %q, want explanation", got)
	}
	if got := UnpinUnsupportedExplanation(); !strings.Contains(got, "not supported") {
		t.Errorf("UnpinUnsupportedExplanation() = %q, want explanation", got)
	}
}
