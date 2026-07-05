package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mariodrengner/gh-repo-cleanup/internal/cleanup"
)

func fakeExec(executed *[]cleanup.Task) ExecFunc {
	return func(tasks []cleanup.Task, progress func(cleanup.Result)) []cleanup.Result {
		*executed = append(*executed, tasks...)
		var out []cleanup.Result
		for _, t := range tasks {
			r := cleanup.Result{Repo: t.Repo.NameWithOwner, Do: t.Do, BundlePath: "/tmp/b.bundle"}
			if progress != nil {
				progress(r)
			}
			out = append(out, r)
		}
		return out
	}
}

// drive pumps messages through App.Update like the Bubble Tea runtime would,
// executing returned commands synchronously.
func drive(t *testing.T, app *App, msgs ...tea.Msg) {
	t.Helper()
	for _, msg := range msgs {
		model, cmd := app.Update(msg)
		*app = *(model.(*App))
		for cmd != nil {
			next := cmd()
			if next == nil {
				break
			}
			if _, isQuit := next.(tea.QuitMsg); isQuit {
				return
			}
			model, cmd = app.Update(next)
			*app = *(model.(*App))
		}
	}
}

func TestFullFlowArchiveAndDelete(t *testing.T) {
	var executed []cleanup.Task
	app := NewApp(cands(), true, fakeExec(&executed))

	drive(t, app, key("d"))              // mark row 0 delete
	drive(t, app, key("down"), key("a")) // mark row 1 archive
	drive(t, app, key("enter"))          // to confirmation
	for _, r := range "delete 1" {
		if r == ' ' {
			drive(t, app, tea.KeyMsg{Type: tea.KeySpace})
		} else {
			drive(t, app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	drive(t, app, tea.KeyMsg{Type: tea.KeyEnter}) // confirm → executes synchronously via drive

	if len(executed) != 2 {
		t.Fatalf("want 2 executed tasks, got %+v", executed)
	}
	view := app.View()
	for _, want := range []string{"✓", "Restore", "b.bundle", "unarchive"} {
		if !strings.Contains(view, want) {
			t.Errorf("done view missing %q:\n%s", want, view)
		}
	}
}

func TestNoMarksMeansNoExecution(t *testing.T) {
	var executed []cleanup.Task
	app := NewApp(cands(), true, fakeExec(&executed))
	drive(t, app, key("enter")) // continue with zero marks
	if len(executed) != 0 {
		t.Fatalf("nothing was marked, nothing may execute: %+v", executed)
	}
	if !strings.Contains(app.View(), "Nothing selected") {
		t.Errorf("view should say nothing selected:\n%s", app.View())
	}
}

func TestEscFromConfirmReturnsToSelection(t *testing.T) {
	var executed []cleanup.Task
	app := NewApp(cands(), true, fakeExec(&executed))
	drive(t, app, key("a"), key("enter"))
	drive(t, app, tea.KeyMsg{Type: tea.KeyEsc})
	if app.Phase() != PhaseSelect {
		t.Fatal("esc must return to selection")
	}
	if len(executed) != 0 {
		t.Fatal("nothing may execute on esc")
	}
}
