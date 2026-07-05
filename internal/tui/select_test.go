package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func cands() []scan.Candidate {
	return []scan.Candidate{
		{Repo: scan.Repo{Name: "a"}, Suggested: scan.ActionDelete,
			Reasons: []string{"orphaned fork"}, Warnings: []string{"3 star(s)"}},
		{Repo: scan.Repo{Name: "b"}, Suggested: scan.ActionArchive,
			Reasons: []string{"inactive"}},
	}
}

func TestNothingIsPreselected(t *testing.T) {
	m := NewSelect(cands(), true)
	if len(m.Marks) != 0 {
		t.Fatalf("marks must start empty, got %v", m.Marks)
	}
}

func TestMarkArchiveDeleteAndUnmark(t *testing.T) {
	m := NewSelect(cands(), true)
	m, _ = m.Update(key("a"))
	if m.Marks[0] != scan.ActionArchive {
		t.Fatalf("want archive mark, got %v", m.Marks)
	}
	m, _ = m.Update(key("d"))
	if m.Marks[0] != scan.ActionDelete {
		t.Fatalf("d must override to delete, got %v", m.Marks)
	}
	m, _ = m.Update(key("space"))
	if _, ok := m.Marks[0]; ok {
		t.Fatalf("space must unmark, got %v", m.Marks)
	}
}

func TestMarkTogglesOff(t *testing.T) {
	m := NewSelect(cands(), true)
	m, _ = m.Update(key("a"))
	m, _ = m.Update(key("a"))
	if _, ok := m.Marks[0]; ok {
		t.Fatal("pressing a twice must unmark")
	}
}

func TestCursorMovesAndClamps(t *testing.T) {
	m := NewSelect(cands(), true)
	m, _ = m.Update(key("up"))
	if m.Cursor != 0 {
		t.Fatal("cursor must clamp at top")
	}
	m, _ = m.Update(key("down"))
	if m.Cursor != 1 {
		t.Fatal("down must move cursor")
	}
	m, _ = m.Update(key("down"))
	if m.Cursor != 1 {
		t.Fatal("cursor must clamp at bottom")
	}
	m, _ = m.Update(key("d"))
	if m.Marks[1] != scan.ActionDelete {
		t.Fatal("mark must apply to cursor row")
	}
}

func TestDeleteDisabledWithoutScope(t *testing.T) {
	m := NewSelect(cands(), false)
	m, _ = m.Update(key("d"))
	if len(m.Marks) != 0 {
		t.Fatal("d must be inert without delete scope")
	}
	if !strings.Contains(m.Note, "gh auth refresh -s delete_repo") {
		t.Fatalf("note must show the scope hint, got %q", m.Note)
	}
}

func TestEnterAndQuit(t *testing.T) {
	m := NewSelect(cands(), true)
	m, _ = m.Update(key("enter"))
	if !m.Done {
		t.Fatal("enter must set Done")
	}
	m2 := NewSelect(cands(), true)
	m2, _ = m2.Update(key("q"))
	if !m2.Aborted {
		t.Fatal("q must set Aborted")
	}
}

func TestViewShowsMarksReasonsWarnings(t *testing.T) {
	m := NewSelect(cands(), true)
	m, _ = m.Update(key("d"))
	view := m.View()
	for _, want := range []string{"[D]", "orphaned fork", "⚠ 3 star(s)", "[a] archive"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

// sorterCands returns candidates with distinct PushedAt so sort order is observable.
// "z-old" is older (index 0 in SortDate), "a-new" is newer (index 1 in SortDate).
// In SortName order they reverse: "a-new"=0, "z-old"=1.
func sorterCands() []scan.Candidate {
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)
	return []scan.Candidate{
		{Repo: scan.Repo{Name: "z-old", PushedAt: old}, Suggested: scan.ActionArchive,
			Reasons: []string{"inactive"}},
		{Repo: scan.Repo{Name: "a-new", PushedAt: newer}, Suggested: scan.ActionArchive,
			Reasons: []string{"inactive"}},
	}
}

func TestSortCyclesMode(t *testing.T) {
	m := NewSelect(sorterCands(), true)
	if m.Sort != scan.SortDate {
		t.Fatalf("initial sort must be SortDate, got %v", m.Sort)
	}
	m, _ = m.Update(key("s"))
	if m.Sort != scan.SortSafety {
		t.Fatalf("after 1st s: want SortSafety, got %v", m.Sort)
	}
	m, _ = m.Update(key("s"))
	if m.Sort != scan.SortName {
		t.Fatalf("after 2nd s: want SortName, got %v", m.Sort)
	}
	m, _ = m.Update(key("s"))
	if m.Sort != scan.SortDate {
		t.Fatalf("after 3rd s: want SortDate (wrap), got %v", m.Sort)
	}
}

func TestSortMarksFollowRepo(t *testing.T) {
	// sorterCands in SortDate order: "z-old"=0, "a-new"=1
	m := NewSelect(sorterCands(), true)
	// move cursor to "a-new" (index 1) and mark it
	m, _ = m.Update(key("down"))
	m, _ = m.Update(key("a"))
	if m.Marks[1] != scan.ActionArchive {
		t.Fatalf("pre-sort: want Marks[1]=archive, got %v", m.Marks)
	}
	// press s → SortSafety (both inactive repos, same group, sort by name): "a-new"=0, "z-old"=1
	m, _ = m.Update(key("s"))
	// "a-new" is now at index 0
	if m.Marks[0] != scan.ActionArchive {
		t.Fatalf("after sort: mark must follow 'a-new' to index 0, got %v", m.Marks)
	}
	if _, stale := m.Marks[1]; stale {
		t.Fatalf("after sort: old index 1 must not have a mark, got %v", m.Marks)
	}
	if m.Cands[0].Repo.Name != "a-new" {
		t.Fatalf("after SortSafety 'a-new' must be at index 0, got %q", m.Cands[0].Repo.Name)
	}
}

func TestSortMarksSurviveFullCycle(t *testing.T) {
	// sorterCands in SortDate order: "z-old"=0, "a-new"=1
	m := NewSelect(sorterCands(), true)
	// move cursor to "a-new" (index 1) and mark it
	m, _ = m.Update(key("down"))
	m, _ = m.Update(key("a"))

	// Press s three times: date → safety → name → date
	// After each press, mark should follow "a-new" to its new index
	// First press: s → SortSafety
	m, _ = m.Update(key("s"))
	if m.Sort != scan.SortSafety {
		t.Fatalf("after 1st s: want SortSafety, got %v", m.Sort)
	}
	// Find "a-new" index and verify mark follows it
	idx := findCandidateIndex(m.Cands, "a-new")
	if m.Marks[idx] != scan.ActionArchive {
		t.Fatalf("after SortSafety: mark must follow 'a-new' to index %d, got %v", idx, m.Marks)
	}

	// Second press: s → SortName
	m, _ = m.Update(key("s"))
	if m.Sort != scan.SortName {
		t.Fatalf("after 2nd s: want SortName, got %v", m.Sort)
	}
	// Find "a-new" index and verify mark follows it
	idx = findCandidateIndex(m.Cands, "a-new")
	if m.Marks[idx] != scan.ActionArchive {
		t.Fatalf("after SortName: mark must follow 'a-new' to index %d, got %v", idx, m.Marks)
	}

	// Third press: s → SortDate (wrap around)
	m, _ = m.Update(key("s"))
	if m.Sort != scan.SortDate {
		t.Fatalf("after 3rd s: want SortDate (wrap), got %v", m.Sort)
	}
	// Find "a-new" index and verify mark follows it
	idx = findCandidateIndex(m.Cands, "a-new")
	if m.Marks[idx] != scan.ActionArchive {
		t.Fatalf("after SortDate: mark must follow 'a-new' to index %d, got %v", idx, m.Marks)
	}
}

func findCandidateIndex(cands []scan.Candidate, name string) int {
	for i, c := range cands {
		if c.Repo.Name == name {
			return i
		}
	}
	return -1
}

func TestSortCursorFollowsRepo(t *testing.T) {
	// sorterCands in SortDate order: "z-old"=0, "a-new"=1
	m := NewSelect(sorterCands(), true)
	m, _ = m.Update(key("down")) // cursor → 1 ("a-new")
	if m.Cursor != 1 {
		t.Fatalf("pre-sort: cursor must be 1, got %d", m.Cursor)
	}
	// SortSafety: "a-new"=0, "z-old"=1 — cursor must follow "a-new" to 0
	m, _ = m.Update(key("s"))
	if m.Cursor != 0 {
		t.Fatalf("cursor must follow 'a-new' to index 0, got %d", m.Cursor)
	}
}

func TestLegendShowsSortMode(t *testing.T) {
	m := NewSelect(sorterCands(), true)
	view := m.View()
	if !strings.Contains(view, "[s] sort: date") {
		t.Errorf("legend must show '[s] sort: date', got:\n%s", view)
	}
	m, _ = m.Update(key("s"))
	view = m.View()
	if !strings.Contains(view, "[s] sort: safety") {
		t.Errorf("legend must show '[s] sort: safety', got:\n%s", view)
	}
	m, _ = m.Update(key("s"))
	view = m.View()
	if !strings.Contains(view, "[s] sort: name") {
		t.Errorf("legend must show '[s] sort: name', got:\n%s", view)
	}
}
