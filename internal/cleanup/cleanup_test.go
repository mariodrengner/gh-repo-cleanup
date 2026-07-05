package cleanup

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

// realMirror makes deps.Mirror create an actual git mirror so Bundle works.
func realMirror(t *testing.T) func(nwo, dest string) error {
	t.Helper()
	src := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	os.WriteFile(filepath.Join(src, "f"), []byte("x"), 0o644)
	run("add", ".")
	run("commit", "-m", "init")
	return func(nwo, dest string) error {
		return exec.Command("git", "clone", "--mirror", src, dest).Run()
	}
}

func TestArchiveCallsArchiveOnly(t *testing.T) {
	var archived, deleted []string
	deps := Deps{
		Archive: func(n string) error { archived = append(archived, n); return nil },
		Delete:  func(n string) error { deleted = append(deleted, n); return nil },
		Mirror:  func(n, d string) error { t.Fatal("no mirror for archive"); return nil },
	}
	res := Execute([]Task{{Repo: scan.Repo{Name: "r", NameWithOwner: "m/r"}, Do: scan.ActionArchive}}, deps, nil)
	if len(archived) != 1 || archived[0] != "m/r" || len(deleted) != 0 || res[0].Err != nil {
		t.Fatalf("archived=%v deleted=%v res=%+v", archived, deleted, res)
	}
}

func TestDeleteRunsBackupFirstAndSucceeds(t *testing.T) {
	var deleted []string
	deps := Deps{
		Delete:    func(n string) error { deleted = append(deleted, n); return nil },
		Mirror:    realMirror(t),
		BackupDir: t.TempDir(),
		Now:       time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC),
	}
	res := Execute([]Task{{Repo: scan.Repo{Name: "r", NameWithOwner: "m/r"}, Do: scan.ActionDelete}}, deps, nil)
	if res[0].Err != nil {
		t.Fatal(res[0].Err)
	}
	if len(deleted) != 1 {
		t.Fatal("delete not called after successful backup")
	}
	if res[0].BundlePath == "" {
		t.Fatal("bundle path missing from result")
	}
	if _, err := os.Stat(res[0].BundlePath); err != nil {
		t.Fatalf("bundle file missing: %v", err)
	}
}

func TestDeleteIsSkippedWhenBackupFails(t *testing.T) {
	deleteCalled := false
	deps := Deps{
		Delete:    func(n string) error { deleteCalled = true; return nil },
		Mirror:    func(n, d string) error { return errors.New("network down") },
		BackupDir: t.TempDir(),
		Now:       time.Now(),
	}
	res := Execute([]Task{{Repo: scan.Repo{Name: "r", NameWithOwner: "m/r"}, Do: scan.ActionDelete}}, deps, nil)
	if deleteCalled {
		t.Fatal("MUST NOT delete when the backup failed")
	}
	if res[0].Err == nil || !strings.Contains(res[0].Err.Error(), "NOT deleted") {
		t.Fatalf("error must state the repo was not deleted, got %v", res[0].Err)
	}
}

func TestUnknownActionReturnsError(t *testing.T) {
	deps := Deps{}
	res := Execute([]Task{{Repo: scan.Repo{Name: "r", NameWithOwner: "m/r"}, Do: scan.Action("bogus")}}, deps, nil)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Err == nil {
		t.Fatal("expected non-nil Err for unknown action")
	}
	if !strings.Contains(res[0].Err.Error(), "unknown action") {
		t.Fatalf("expected error to contain 'unknown action', got %v", res[0].Err)
	}
}

func TestBatchContinuesAfterFailureAndReportsProgress(t *testing.T) {
	var progressed []string
	deps := Deps{
		Archive: func(n string) error {
			if n == "m/bad" {
				return errors.New("boom")
			}
			return nil
		},
	}
	tasks := []Task{
		{Repo: scan.Repo{Name: "bad", NameWithOwner: "m/bad"}, Do: scan.ActionArchive},
		{Repo: scan.Repo{Name: "good", NameWithOwner: "m/good"}, Do: scan.ActionArchive},
	}
	res := Execute(tasks, deps, func(r Result) { progressed = append(progressed, r.Repo) })
	if len(res) != 2 || res[0].Err == nil || res[1].Err != nil {
		t.Fatalf("batch must continue after failure: %+v", res)
	}
	if len(progressed) != 2 {
		t.Fatalf("progress callback: got %v", progressed)
	}
}
