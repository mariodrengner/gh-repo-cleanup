package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gh "github.com/cli/go-gh/v2"
	"github.com/mariodrengner/gh-repo-cleanup/internal/cleanup"
	"github.com/mariodrengner/gh-repo-cleanup/internal/github"
	"github.com/mariodrengner/gh-repo-cleanup/internal/report"
	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
	"github.com/mariodrengner/gh-repo-cleanup/internal/tui"
)

func main() {
	list := flag.Bool("list", false, "print the candidate report and exit (read-only)")
	jsonOut := flag.Bool("json", false, "print candidates as JSON and exit (read-only)")
	olderThan := flag.String("older-than", "1y", "inactivity threshold (e.g. 90d, 6m, 1y)")
	flag.Parse()

	threshold, err := scan.ParseThreshold(*olderThan)
	if err != nil {
		fatal(err)
	}
	client, err := github.New()
	if err != nil {
		fatal(fmt.Errorf("gh authentication required (run `gh auth login`): %w", err))
	}

	_, cands, err := runScan(client, threshold)
	if err != nil {
		fatal(err)
	}

	switch {
	case *jsonOut:
		if err := report.JSON(os.Stdout, cands); err != nil {
			fatal(err)
		}
	case *list:
		report.Text(os.Stdout, cands)
	default:
		if len(cands) == 0 {
			report.Text(os.Stdout, cands)
			return
		}
		canDelete, err := client.HasDeleteScope()
		if err != nil {
			fatal(err)
		}
		if !canDelete {
			fmt.Fprintln(os.Stderr,
				"note: token lacks the delete_repo scope — deletions are disabled.\n"+
					"      enable with: gh auth refresh -s delete_repo")
		}
		deps := cleanup.Deps{
			Archive:   client.Archive,
			Delete:    client.Delete,
			Mirror:    mirrorViaGh,
			BackupDir: backupDir(),
			Now:       time.Now(),
		}
		app := tui.NewApp(cands, canDelete,
			func(tasks []cleanup.Task, progress func(cleanup.Result)) []cleanup.Result {
				return cleanup.Execute(tasks, deps, progress)
			})
		if _, err := tea.NewProgram(app).Run(); err != nil {
			fatal(err)
		}
	}
}

func runScan(client *github.Client, olderThan time.Duration) (string, []scan.Candidate, error) {
	login, err := client.Username()
	if err != nil {
		return "", nil, err
	}
	repos, err := client.ListRepos()
	if err != nil {
		return "", nil, err
	}
	prTargets, err := client.OpenPRTargets(login)
	if err != nil {
		return "", nil, err
	}
	forks := 0
	for _, r := range repos {
		if r.IsFork && !r.IsArchived {
			forks++
		}
	}
	done := 0
	for i := range repos {
		if !repos[i].IsFork || repos[i].IsArchived {
			continue
		}
		done++
		fmt.Fprintf(os.Stderr, "\rScanning %d repos — checking fork %d/%d …", len(repos), done, forks)
		if err := client.EnrichFork(login, &repos[i]); err != nil {
			return "", nil, err
		}
		repos[i].HasOpenUpstreamPR = prTargets[repos[i].Parent]
	}
	if forks > 0 {
		fmt.Fprint(os.Stderr, "\r\033[K")
	}
	return login, scan.Evaluate(repos, scan.Options{OlderThan: olderThan}), nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

// mirrorViaGh clones a mirror through gh so auth and git protocol just work.
func mirrorViaGh(nameWithOwner, dest string) error {
	_, stderr, err := gh.Exec("repo", "clone", nameWithOwner, dest, "--", "--mirror")
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

func backupDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fatal(err)
	}
	return filepath.Join(home, ".local", "share", "gh-repo-cleanup", "backups")
}
