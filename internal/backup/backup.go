// Package backup writes a git bundle plus repo metadata before a deletion.
package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

type Meta struct {
	Name          string    `json:"name"`
	NameWithOwner string    `json:"name_with_owner"`
	Description   string    `json:"description,omitempty"`
	Homepage      string    `json:"homepage,omitempty"`
	Topics        []string  `json:"topics,omitempty"`
	IsFork        bool      `json:"is_fork"`
	Parent        string    `json:"parent,omitempty"`
	DeletedAt     time.Time `json:"deleted_at"`
}

func MetaFor(r scan.Repo) Meta {
	return Meta{
		Name:          r.Name,
		NameWithOwner: r.NameWithOwner,
		Description:   r.Description,
		Homepage:      r.Homepage,
		Topics:        r.Topics,
		IsFork:        r.IsFork,
		Parent:        r.Parent,
	}
}

// Bundle creates <destBase>/<name>-<date>/<name>.bundle plus metadata.json
// from an existing git mirror directory. On any error nothing must be
// treated as backed up.
func Bundle(mirrorDir, destBase string, meta Meta, now time.Time) (string, error) {
	dir := filepath.Join(destBase, fmt.Sprintf("%s-%s", meta.Name, now.Format("2006-01-02")))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	bundle := filepath.Join(dir, meta.Name+".bundle")
	cmd := exec.Command("git", "-C", mirrorDir, "bundle", "create", bundle, "--all")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git bundle create: %w\n%s", err, out)
	}
	meta.DeletedAt = now
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), raw, 0o644); err != nil {
		return "", err
	}
	return bundle, nil
}
