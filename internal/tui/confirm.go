package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

type ConfirmModel struct {
	Archives []scan.Candidate
	Deletes  []scan.Candidate
	Input    string
	Note     string
	Done     bool
	Back     bool
}

func NewConfirm(cands []scan.Candidate, marks map[int]scan.Action) ConfirmModel {
	var m ConfirmModel
	for i, c := range cands {
		switch marks[i] {
		case scan.ActionArchive:
			m.Archives = append(m.Archives, c)
		case scan.ActionDelete:
			m.Deletes = append(m.Deletes, c)
		}
	}
	return m
}

func (m ConfirmModel) phrase() string { return fmt.Sprintf("delete %d", len(m.Deletes)) }

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.Back = true
	case tea.KeyEnter:
		if len(m.Deletes) == 0 || m.Input == m.phrase() {
			m.Done = true
		} else {
			m.Note = fmt.Sprintf("type %q to confirm the deletions", m.phrase())
		}
	case tea.KeyBackspace:
		if len(m.Input) > 0 {
			m.Input = m.Input[:len(m.Input)-1]
		}
	case tea.KeySpace:
		m.Input += " "
	case tea.KeyRunes:
		m.Input += string(keyMsg.Runes)
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	var b strings.Builder
	b.WriteString("Summary\n\n")
	if len(m.Archives) > 0 {
		fmt.Fprintf(&b, "Archive (%d) — reversible:\n", len(m.Archives))
		for _, c := range m.Archives {
			fmt.Fprintf(&b, "  • %s\n", c.Repo.NameWithOwner)
		}
		b.WriteString("\n")
	}
	if len(m.Deletes) > 0 {
		fmt.Fprintf(&b, "Delete (%d) — a backup bundle is written first:\n", len(m.Deletes))
		for _, c := range m.Deletes {
			fmt.Fprintf(&b, "  • %s\n", c.Repo.NameWithOwner)
		}
		fmt.Fprintf(&b, "\nType %q and press enter: %s▌\n", m.phrase(), m.Input)
	} else {
		b.WriteString("Press enter to confirm.\n")
	}
	if m.Note != "" {
		fmt.Fprintf(&b, "\n  %s\n", m.Note)
	}
	b.WriteString("\n[esc] back\n")
	return b.String()
}
