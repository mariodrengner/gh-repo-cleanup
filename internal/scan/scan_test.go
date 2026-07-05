package scan

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

func old() time.Time    { return now.AddDate(-2, 0, 0) } // 2 years ago
func recent() time.Time { return now.AddDate(0, -1, 0) } // 1 month ago

func eval(t *testing.T, repos ...Repo) []Candidate {
	t.Helper()
	return Evaluate(repos, Options{Now: now}) // default threshold 12 months
}

func TestOrphanedForkIsDeleteCandidate(t *testing.T) {
	c := eval(t, Repo{Name: "f", IsFork: true, AheadBy: 0, PushedAt: recent()})
	if len(c) != 1 || c[0].Suggested != ActionDelete {
		t.Fatalf("want 1 delete candidate, got %+v", c)
	}
	if !strings.Contains(c[0].Reasons[0], "no own commits") {
		t.Errorf("reason should explain orphaned fork, got %q", c[0].Reasons[0])
	}
}

func TestForkWithOpenUpstreamPRIsNeverACandidate(t *testing.T) {
	c := eval(t, Repo{Name: "f", IsFork: true, AheadBy: 0, PushedAt: old(), HasOpenUpstreamPR: true})
	if len(c) != 0 {
		t.Fatalf("fork with open upstream PR must be protected, got %+v", c)
	}
}

func TestInactiveForkWithOwnCommitsWarnsAboutThem(t *testing.T) {
	c := eval(t, Repo{Name: "f", IsFork: true, AheadBy: 3, PushedAt: old()})
	if len(c) != 1 || c[0].Suggested != ActionDelete {
		t.Fatalf("want 1 delete candidate, got %+v", c)
	}
	if !hasWarning(c[0], "3 own commit") {
		t.Errorf("want own-commits warning, got %v", c[0].Warnings)
	}
}

func TestActiveForkWithOwnCommitsIsNotACandidate(t *testing.T) {
	c := eval(t, Repo{Name: "f", IsFork: true, AheadBy: 3, PushedAt: recent()})
	if len(c) != 0 {
		t.Fatalf("active fork with own commits must be kept, got %+v", c)
	}
}

func TestForkWithUnknownComparisonWarns(t *testing.T) {
	c := eval(t, Repo{Name: "f", IsFork: true, AheadBy: AheadUnknown, PushedAt: old()})
	if len(c) != 1 || !hasWarning(c[0], "could not compare") {
		t.Fatalf("want unknown-comparison warning, got %+v", c)
	}
}

func TestInactiveOwnRepoSuggestsArchiveNeverDelete(t *testing.T) {
	c := eval(t, Repo{Name: "r", PushedAt: old()})
	if len(c) != 1 || c[0].Suggested != ActionArchive {
		t.Fatalf("want archive suggestion, got %+v", c)
	}
}

func TestActiveOwnRepoIsNotACandidate(t *testing.T) {
	if c := eval(t, Repo{Name: "r", PushedAt: recent()}); len(c) != 0 {
		t.Fatalf("active repo must be kept, got %+v", c)
	}
}

func TestArchivedReposAreSkipped(t *testing.T) {
	if c := eval(t, Repo{Name: "r", IsArchived: true, PushedAt: old()}); len(c) != 0 {
		t.Fatalf("archived repo must be skipped, got %+v", c)
	}
}

func TestProtectionSignalsBecomeWarningsNotExclusions(t *testing.T) {
	c := eval(t, Repo{Name: "r", PushedAt: old(), Stars: 5, Forks: 2, OpenIssues: 1})
	if len(c) != 1 {
		t.Fatalf("repo with stars stays a candidate, got %+v", c)
	}
	for _, want := range []string{"5 star", "forked 2", "1 open issue"} {
		if !hasWarning(c[0], want) {
			t.Errorf("missing warning %q in %v", want, c[0].Warnings)
		}
	}
}

func TestCustomThreshold(t *testing.T) {
	fiveMonths := now.AddDate(0, -5, 0)
	c := Evaluate([]Repo{{Name: "r", PushedAt: fiveMonths}},
		Options{Now: now, OlderThan: 90 * 24 * time.Hour})
	if len(c) != 1 {
		t.Fatalf("5-month-old repo with 90d threshold must be a candidate, got %+v", c)
	}
}

func TestSortingOrphansFirstThenStaleForksThenInactive(t *testing.T) {
	c := eval(t,
		Repo{Name: "z-inactive", PushedAt: old()},
		Repo{Name: "a-stale-fork", IsFork: true, AheadBy: 2, PushedAt: old()},
		Repo{Name: "m-orphan", IsFork: true, AheadBy: 0, PushedAt: recent()},
	)
	Sort(c, SortSafety)
	got := []string{c[0].Repo.Name, c[1].Repo.Name, c[2].Repo.Name}
	want := []string{"m-orphan", "a-stale-fork", "z-inactive"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order: got %v, want %v", got, want)
		}
	}
}

func TestEvaluateDefaultSortIsOldestFirst(t *testing.T) {
	oldest := now.AddDate(-3, 0, 0)
	middle := now.AddDate(-2, 0, 0)
	newest := now.AddDate(-1, -1, 0) // 13 months ago (still qualifies)
	c := eval(t,
		Repo{Name: "b-mid", PushedAt: middle},
		Repo{Name: "c-new", PushedAt: newest},
		Repo{Name: "a-old", PushedAt: oldest},
	)
	if len(c) != 3 {
		t.Fatalf("want 3 candidates, got %d", len(c))
	}
	got := []string{c[0].Repo.Name, c[1].Repo.Name, c[2].Repo.Name}
	want := []string{"a-old", "b-mid", "c-new"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("default sort must be oldest-first: got %v, want %v", got, want)
		}
	}
}

func TestSortName(t *testing.T) {
	c := []Candidate{
		{Repo: Repo{Name: "zebra", PushedAt: old()}},
		{Repo: Repo{Name: "apple", PushedAt: old()}},
		{Repo: Repo{Name: "mango", PushedAt: old()}},
	}
	Sort(c, SortName)
	got := []string{c[0].Repo.Name, c[1].Repo.Name, c[2].Repo.Name}
	want := []string{"apple", "mango", "zebra"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortName: got %v, want %v", got, want)
		}
	}
}

func TestSortDateTieBreak(t *testing.T) {
	sameTime := old()
	c := []Candidate{
		{Repo: Repo{Name: "z-repo", PushedAt: sameTime}},
		{Repo: Repo{Name: "a-repo", PushedAt: sameTime}},
	}
	Sort(c, SortDate)
	if c[0].Repo.Name != "a-repo" || c[1].Repo.Name != "z-repo" {
		t.Fatalf("tie-break by name: got %v %v", c[0].Repo.Name, c[1].Repo.Name)
	}
}

func hasWarning(c Candidate, substr string) bool {
	for _, w := range c.Warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}
