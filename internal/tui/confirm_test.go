package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

func confirmFixture(marks map[int]scan.Action) ConfirmModel {
	return NewConfirm(cands(), marks) // cands() from select_test.go: index 0 and 1
}

func typeString(m ConfirmModel, s string) ConfirmModel {
	for _, r := range s {
		if r == ' ' {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
		} else {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	return m
}

func TestGroupsByAction(t *testing.T) {
	m := confirmFixture(map[int]scan.Action{0: scan.ActionDelete, 1: scan.ActionArchive})
	if len(m.Deletes) != 1 || len(m.Archives) != 1 {
		t.Fatalf("grouping wrong: %+v", m)
	}
}

func TestArchiveOnlyConfirmsWithPlainEnter(t *testing.T) {
	m := confirmFixture(map[int]scan.Action{1: scan.ActionArchive})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Done {
		t.Fatal("archive-only must confirm on enter")
	}
}

func TestDeleteRequiresTypedPhrase(t *testing.T) {
	m := confirmFixture(map[int]scan.Action{0: scan.ActionDelete})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Done {
		t.Fatal("enter alone must NOT confirm deletions")
	}
	if !strings.Contains(m.Note, `"delete 1"`) {
		t.Fatalf("note must show required phrase, got %q", m.Note)
	}
	m = typeString(m, "delete 1")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Done {
		t.Fatal("typed phrase + enter must confirm")
	}
}

func TestWrongPhraseDoesNotConfirm(t *testing.T) {
	m := confirmFixture(map[int]scan.Action{0: scan.ActionDelete})
	m = typeString(m, "delete 9")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Done {
		t.Fatal("wrong phrase must not confirm")
	}
}

func TestBackspaceEditsInput(t *testing.T) {
	m := confirmFixture(map[int]scan.Action{0: scan.ActionDelete})
	m = typeString(m, "xx")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.Input != "x" {
		t.Fatalf("backspace: got %q", m.Input)
	}
}

func TestEscGoesBack(t *testing.T) {
	m := confirmFixture(map[int]scan.Action{0: scan.ActionDelete})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Back {
		t.Fatal("esc must set Back")
	}
}

func TestViewListsGroupsAndPrompt(t *testing.T) {
	m := confirmFixture(map[int]scan.Action{0: scan.ActionDelete, 1: scan.ActionArchive})
	view := m.View()
	for _, want := range []string{"Archive (1)", "Delete (1)", "delete 1", "backup"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}
