package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mariodrengner/gh-repo-cleanup/internal/github"
	"github.com/mariodrengner/gh-repo-cleanup/internal/report"
	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
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
		// Interactive TUI is wired in a later task.
		report.Text(os.Stdout, cands)
		fmt.Fprintln(os.Stderr, "\n(interactive mode not implemented yet — read-only report shown)")
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
