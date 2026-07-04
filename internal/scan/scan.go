// Package scan holds the domain core: repo data in, explained cleanup
// candidates out. Pure functions, no I/O.
package scan

import (
	"fmt"
	"sort"
	"time"
)

type Action string

const (
	ActionArchive Action = "archive"
	ActionDelete  Action = "delete"
)

// AheadUnknown marks forks where the upstream comparison failed.
const AheadUnknown = -1

type Repo struct {
	Name          string
	NameWithOwner string
	Description   string
	Homepage      string
	Topics        []string
	IsFork        bool
	IsArchived    bool
	PushedAt      time.Time
	Stars         int
	Forks         int
	OpenIssues    int
	DefaultBranch string

	// Fork enrichment; only meaningful when IsFork is true.
	Parent            string // "owner/repo" of upstream
	AheadBy           int    // own commits ahead of upstream, AheadUnknown if unknown
	HasOpenUpstreamPR bool
}

type Candidate struct {
	Repo      Repo
	Suggested Action
	Reasons   []string
	Warnings  []string
}

type Options struct {
	OlderThan time.Duration // zero value = 12 months
	Now       time.Time     // zero value = time.Now()
}

func Evaluate(repos []Repo, opts Options) []Candidate {
	if opts.OlderThan == 0 {
		opts.OlderThan = 365 * 24 * time.Hour
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	cutoff := opts.Now.Add(-opts.OlderThan)

	var orphans, staleForks, inactive []Candidate
	for _, r := range repos {
		if r.IsArchived {
			continue
		}
		if r.IsFork {
			if r.HasOpenUpstreamPR {
				continue
			}
			switch {
			case r.AheadBy == 0:
				orphans = append(orphans, Candidate{
					Repo:      r,
					Suggested: ActionDelete,
					Reasons:   []string{"orphaned fork: no own commits ahead of upstream"},
					Warnings:  protectionWarnings(r),
				})
			case r.PushedAt.Before(cutoff):
				c := Candidate{
					Repo:      r,
					Suggested: ActionDelete,
					Reasons:   []string{fmt.Sprintf("inactive fork: last push %s", r.PushedAt.Format("2006-01-02"))},
					Warnings:  protectionWarnings(r),
				}
				if r.AheadBy == AheadUnknown {
					c.Warnings = append(c.Warnings, "could not compare with upstream — own commits unknown")
				} else {
					c.Warnings = append(c.Warnings,
						fmt.Sprintf("fork has %d own commit(s) — the backup will preserve them", r.AheadBy))
				}
				staleForks = append(staleForks, c)
			}
			continue
		}
		if r.PushedAt.Before(cutoff) {
			inactive = append(inactive, Candidate{
				Repo:      r,
				Suggested: ActionArchive,
				Reasons:   []string{fmt.Sprintf("inactive: last push %s", r.PushedAt.Format("2006-01-02"))},
				Warnings:  protectionWarnings(r),
			})
		}
	}

	for _, group := range [][]Candidate{orphans, staleForks, inactive} {
		sort.Slice(group, func(i, j int) bool { return group[i].Repo.Name < group[j].Repo.Name })
	}
	out := append(orphans, staleForks...)
	return append(out, inactive...)
}

func protectionWarnings(r Repo) []string {
	var w []string
	if r.Stars > 0 {
		w = append(w, fmt.Sprintf("%d star(s)", r.Stars))
	}
	if r.Forks > 0 {
		w = append(w, fmt.Sprintf("forked %d time(s) by others", r.Forks))
	}
	if r.OpenIssues > 0 {
		w = append(w, fmt.Sprintf("%d open issue(s)/PR(s)", r.OpenIssues))
	}
	return w
}
