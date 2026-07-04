// Package cleanup orchestrates archive/delete actions with mandatory
// backup-before-delete. All side effects are injected via Deps.
package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mariodrengner/gh-repo-cleanup/internal/backup"
	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

type Task struct {
	Repo scan.Repo
	Do   scan.Action
}

type Result struct {
	Repo       string
	Do         scan.Action
	BundlePath string
	Err        error
}

type Deps struct {
	Archive   func(nameWithOwner string) error
	Delete    func(nameWithOwner string) error
	Mirror    func(nameWithOwner, destDir string) error
	BackupDir string
	Now       time.Time
}

func Execute(tasks []Task, deps Deps, progress func(Result)) []Result {
	results := make([]Result, 0, len(tasks))
	for _, t := range tasks {
		r := run(t, deps)
		if progress != nil {
			progress(r)
		}
		results = append(results, r)
	}
	return results
}

func run(t Task, deps Deps) Result {
	res := Result{Repo: t.Repo.NameWithOwner, Do: t.Do}
	switch t.Do {
	case scan.ActionArchive:
		res.Err = deps.Archive(t.Repo.NameWithOwner)
	case scan.ActionDelete:
		tmp, err := os.MkdirTemp("", "gh-repo-cleanup-*")
		if err != nil {
			res.Err = err
			return res
		}
		defer os.RemoveAll(tmp)
		mirror := filepath.Join(tmp, "mirror.git")
		if err := deps.Mirror(t.Repo.NameWithOwner, mirror); err != nil {
			res.Err = fmt.Errorf("backup failed, repo NOT deleted: %w", err)
			return res
		}
		bundle, err := backup.Bundle(mirror, deps.BackupDir, backup.MetaFor(t.Repo), deps.Now)
		if err != nil {
			res.Err = fmt.Errorf("backup failed, repo NOT deleted: %w", err)
			return res
		}
		res.BundlePath = bundle
		res.Err = deps.Delete(t.Repo.NameWithOwner)
	}
	return res
}
