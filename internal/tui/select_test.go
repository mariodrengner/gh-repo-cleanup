package tui

import (
	"strings"
	"testing"

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
