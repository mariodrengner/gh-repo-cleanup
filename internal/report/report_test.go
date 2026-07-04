package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

func fixture() []scan.Candidate {
	return []scan.Candidate{
		{
			Repo: scan.Repo{Name: "old-fork", IsFork: true, Stars: 0,
				PushedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)},
			Suggested: scan.ActionDelete,
			Reasons:   []string{"orphaned fork: no own commits ahead of upstream"},
		},
		{
			Repo: scan.Repo{Name: "old-repo", Stars: 3,
				PushedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
			Suggested: scan.ActionArchive,
			Reasons:   []string{"inactive: last push 2024-06-01"},
			Warnings:  []string{"3 star(s)"},
		},
	}
}

func TestTextContainsAllColumnsAndWarnings(t *testing.T) {
	var b bytes.Buffer
	Text(&b, fixture())
	out := b.String()
	for _, want := range []string{"old-fork", "fork", "2025-01-02", "delete",
		"old-repo", "archive", "orphaned fork", "⚠ 3 star(s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestTextEmpty(t *testing.T) {
	var b bytes.Buffer
	Text(&b, nil)
	if !strings.Contains(b.String(), "No cleanup candidates") {
		t.Errorf("empty message missing, got %q", b.String())
	}
}

func TestJSONRoundTrips(t *testing.T) {
	var b bytes.Buffer
	if err := JSON(&b, fixture()); err != nil {
		t.Fatal(err)
	}
	var got []scan.Candidate
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Repo.Name != "old-fork" || got[1].Suggested != scan.ActionArchive {
		t.Fatalf("round trip wrong: %+v", got)
	}
}
