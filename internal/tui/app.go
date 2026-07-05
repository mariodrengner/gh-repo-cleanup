package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mariodrengner/gh-repo-cleanup/internal/cleanup"
	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

type Phase int

const (
	PhaseSelect Phase = iota
	PhaseConfirm
	PhaseExec
	PhaseDone
)

type ExecFunc func(tasks []cleanup.Task, progress func(cleanup.Result)) []cleanup.Result

type App struct {
	sel     SelectModel
	conf    ConfirmModel
	phase   Phase
	exec    ExecFunc
	results []cleanup.Result
	total   int
	resCh   chan cleanup.Result
	nothing bool
}

type resultMsg cleanup.Result
type execDoneMsg struct{}

func NewApp(cands []scan.Candidate, canDelete bool, exec ExecFunc) *App {
	return &App{sel: NewSelect(cands, canDelete), exec: exec}
}

func (a *App) Phase() Phase  { return a.phase }
func (a *App) Init() tea.Cmd { return nil }

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch a.phase {
	case PhaseSelect:
		var cmd tea.Cmd
		a.sel, cmd = a.sel.Update(msg)
		if a.sel.Aborted {
			return a, tea.Quit
		}
		if a.sel.Done {
			a.sel.Done = false
			if len(a.sel.Marks) == 0 {
				a.nothing = true
				a.phase = PhaseDone
				return a, nil
			}
			a.conf = NewConfirm(a.sel.Cands, a.sel.Marks)
			a.phase = PhaseConfirm
		}
		return a, cmd
	case PhaseConfirm:
		var cmd tea.Cmd
		a.conf, cmd = a.conf.Update(msg)
		if a.conf.Back {
			a.phase = PhaseSelect
			return a, nil
		}
		if a.conf.Done {
			a.phase = PhaseExec
			return a, a.startExec()
		}
		return a, cmd
	case PhaseExec:
		switch m := msg.(type) {
		case resultMsg:
			a.results = append(a.results, cleanup.Result(m))
			return a, waitForResult(a.resCh)
		case execDoneMsg:
			a.phase = PhaseDone
		}
		return a, nil
	default: // PhaseDone
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Quit
		}
		return a, nil
	}
}

func (a *App) startExec() tea.Cmd {
	var tasks []cleanup.Task
	for _, c := range a.conf.Archives {
		tasks = append(tasks, cleanup.Task{Repo: c.Repo, Do: scan.ActionArchive})
	}
	for _, c := range a.conf.Deletes {
		tasks = append(tasks, cleanup.Task{Repo: c.Repo, Do: scan.ActionDelete})
	}
	a.total = len(tasks)
	ch := make(chan cleanup.Result)
	a.resCh = ch
	go func() {
		a.exec(tasks, func(r cleanup.Result) { ch <- r })
		close(ch)
	}()
	return waitForResult(ch)
}

func waitForResult(ch <-chan cleanup.Result) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return execDoneMsg{}
		}
		return resultMsg(r)
	}
}

func (a *App) View() string {
	switch a.phase {
	case PhaseSelect:
		return a.sel.View()
	case PhaseConfirm:
		return a.conf.View()
	case PhaseExec:
		return fmt.Sprintf("Working … %d/%d done\n%s", len(a.results), a.total, a.resultLines())
	default:
		if a.nothing {
			return "Nothing selected — no changes made.\n\npress any key to exit\n"
		}
		var b strings.Builder
		b.WriteString("Done.\n\n")
		b.WriteString(a.resultLines())
		if a.hasSucceeded(scan.ActionDelete) {
			b.WriteString("\nRestore a deleted repo from its bundle:\n")
			b.WriteString("  git clone <bundle> <name> && cd <name> && gh repo create <name> --private --source=. --push\n")
		}
		if a.hasSucceeded(scan.ActionArchive) {
			b.WriteString("\nTo unarchive: gh repo unarchive <owner>/<repo>\n")
		}
		b.WriteString("\npress any key to exit\n")
		return b.String()
	}
}

func (a *App) resultLines() string {
	var b strings.Builder
	for _, r := range a.results {
		if r.Err != nil {
			fmt.Fprintf(&b, "  ✗ %s %s: %v\n", r.Do, r.Repo, r.Err)
			continue
		}
		fmt.Fprintf(&b, "  ✓ %s %s", r.Do, r.Repo)
		if r.BundlePath != "" {
			fmt.Fprintf(&b, " (backup: %s)", r.BundlePath)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (a *App) hasSucceeded(action scan.Action) bool {
	for _, r := range a.results {
		if r.Do == action && r.Err == nil {
			return true
		}
	}
	return false
}
