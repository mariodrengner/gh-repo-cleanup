package backup

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

// makeMirror creates a real git repo with one commit and a mirror clone of it.
func makeMirror(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	run := func(args ...string) {
		t.Helper()
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
	os.WriteFile(filepath.Join(src, "f.txt"), []byte("x"), 0o644)
	run("add", ".")
	run("commit", "-m", "init")

	mirror := filepath.Join(t.TempDir(), "m.git")
	if out, err := exec.Command("git", "clone", "--mirror", src, mirror).CombinedOutput(); err != nil {
		t.Fatalf("mirror clone: %v\n%s", err, out)
	}
	return mirror
}

func TestBundleCreatesVerifiableBundleAndMetadata(t *testing.T) {
	mirror := makeMirror(t)
	dest := t.TempDir()
	now := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	meta := MetaFor(scan.Repo{
		Name: "myrepo", NameWithOwner: "mario/myrepo",
		Description: "d", Topics: []string{"x"}, IsFork: true, Parent: "up/myrepo",
	})

	bundle, err := Bundle(mirror, dest, meta, now)
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(dest, "myrepo-2026-07-04")
	if !strings.HasPrefix(bundle, wantDir) {
		t.Fatalf("bundle %q not in %q", bundle, wantDir)
	}
	if out, err := exec.Command("git", "bundle", "verify", bundle).CombinedOutput(); err != nil {
		t.Fatalf("bundle not verifiable: %v\n%s", err, out)
	}

	raw, err := os.ReadFile(filepath.Join(wantDir, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got Meta
	json.Unmarshal(raw, &got)
	if got.NameWithOwner != "mario/myrepo" || !got.IsFork || got.Parent != "up/myrepo" ||
		!got.DeletedAt.Equal(now) {
		t.Fatalf("metadata wrong: %+v", got)
	}
}

func TestBundleFailsOnBrokenMirror(t *testing.T) {
	destBase := t.TempDir()
	if _, err := Bundle(t.TempDir(), destBase, Meta{Name: "x"}, time.Now()); err == nil {
		t.Fatal("Bundle on a non-git dir must fail")
	}
	entries, err := os.ReadDir(destBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected destBase to be empty after error, but found %d entries", len(entries))
	}
}
