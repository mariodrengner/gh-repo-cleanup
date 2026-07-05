// Package tui contains the Bubble Tea screens. Models hold no domain logic:
// rendering and key handling only.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

type SelectModel struct {
	Cands     []scan.Candidate
	Marks     map[int]scan.Action
	Cursor    int
	Sort      scan.SortMode
	CanDelete bool
	Note      string
	Done      bool
	Aborted   bool
}

func NewSelect(cands []scan.Candidate, canDelete bool) SelectModel {
	return SelectModel{Cands: cands, Marks: map[int]scan.Action{}, CanDelete: canDelete}
}

func (m SelectModel) Update(msg tea.Msg) (SelectModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	m.Note = ""
	switch keyMsg.String() {
	case "q", "ctrl+c":
		m.Aborted = true
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(m.Cands)-1 {
			m.Cursor++
		}
	case "a":
		m = m.toggle(scan.ActionArchive)
	case "d":
		if m.CanDelete {
			m = m.toggle(scan.ActionDelete)
		} else {
			m.Note = "deletions disabled — run: gh auth refresh -s delete_repo"
		}
	case " ":
		delete(m.Marks, m.Cursor)
	case "s":
		m = m.cycleSort()
	case "enter":
		m.Done = true
	}
	return m, nil
}

func (m SelectModel) cycleSort() SelectModel {
	// Build name-based marks so they survive the re-ordering.
	nameMarks := make(map[string]scan.Action, len(m.Marks))
	for idx, action := range m.Marks {
		nameMarks[m.Cands[idx].Repo.Name] = action
	}
	currentName := ""
	if len(m.Cands) > 0 {
		currentName = m.Cands[m.Cursor].Repo.Name
	}

	// Advance the mode.
	m.Sort = (m.Sort + 1) % (scan.SortName + 1)
	scan.Sort(m.Cands, m.Sort)

	// Rebuild index-based marks and reposition cursor.
	m.Marks = make(map[int]scan.Action, len(nameMarks))
	for i, c := range m.Cands {
		if action, ok := nameMarks[c.Repo.Name]; ok {
			m.Marks[i] = action
		}
		if c.Repo.Name == currentName {
			m.Cursor = i
		}
	}
	return m
}

func (m SelectModel) toggle(a scan.Action) SelectModel {
	if m.Marks[m.Cursor] == a {
		delete(m.Marks, m.Cursor)
	} else {
		m.Marks[m.Cursor] = a
	}
	return m
}

func (m SelectModel) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Candidates (%d) — %d marked\n\n", len(m.Cands), len(m.Marks))
	for i, c := range m.Cands {
		cursor := "  "
		if i == m.Cursor {
			cursor = "▸ "
		}
		mark := "[ ]"
		switch m.Marks[i] {
		case scan.ActionArchive:
			mark = "[A]"
		case scan.ActionDelete:
			mark = "[D]"
		}
		typ := "repo"
		if c.Repo.IsFork {
			typ = "fork"
		}
		fmt.Fprintf(&b, "%s%s %-40s %-4s pushed %s  ★%d\n",
			cursor, mark, c.Repo.Name, typ, c.Repo.PushedAt.Format("2006-01-02"), c.Repo.Stars)
	}
	if len(m.Cands) > 0 {
		cur := m.Cands[m.Cursor]
		fmt.Fprintf(&b, "\n  %s\n", strings.Join(cur.Reasons, "; "))
		for _, w := range cur.Warnings {
			fmt.Fprintf(&b, "  ⚠ %s\n", w)
		}
	}
	if m.Note != "" {
		fmt.Fprintf(&b, "\n  %s\n", m.Note)
	}
	fmt.Fprintf(&b, "\n[a] archive  [d] delete  [space] unmark  [s] sort: %s  [enter] continue  [q] quit\n", m.Sort)
	return b.String()
}
