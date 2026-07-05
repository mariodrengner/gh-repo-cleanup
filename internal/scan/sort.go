package scan

import "sort"

// SortMode controls the ordering of candidates.
type SortMode int

const (
	SortDate   SortMode = iota // PushedAt ascending — oldest first (default)
	SortSafety                 // orphaned forks, then stale forks, then inactive repos; name A-Z within each group
	SortName                   // repo name A-Z
)

// String returns the label shown in the TUI legend: "date", "safety", "name".
func (s SortMode) String() string {
	switch s {
	case SortSafety:
		return "safety"
	case SortName:
		return "name"
	default:
		return "date"
	}
}

// safetyGroup returns the sort priority for SortSafety:
// orphaned fork (IsFork && AheadBy == 0) = 0, stale fork = 1, inactive repo = 2.
func safetyGroup(c Candidate) int {
	if c.Repo.IsFork {
		if c.Repo.AheadBy == 0 {
			return 0
		}
		return 1
	}
	return 2
}

// Sort orders candidates in place according to mode.
func Sort(cands []Candidate, mode SortMode) {
	switch mode {
	case SortSafety:
		sort.SliceStable(cands, func(i, j int) bool {
			gi, gj := safetyGroup(cands[i]), safetyGroup(cands[j])
			if gi != gj {
				return gi < gj
			}
			return cands[i].Repo.Name < cands[j].Repo.Name
		})
	case SortName:
		sort.SliceStable(cands, func(i, j int) bool {
			return cands[i].Repo.Name < cands[j].Repo.Name
		})
	default: // SortDate
		sort.SliceStable(cands, func(i, j int) bool {
			ti, tj := cands[i].Repo.PushedAt, cands[j].Repo.PushedAt
			if ti.Equal(tj) {
				return cands[i].Repo.Name < cands[j].Repo.Name
			}
			return ti.Before(tj)
		})
	}
}
