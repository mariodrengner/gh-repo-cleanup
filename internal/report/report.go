// Package report renders candidates as a text table or JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

// Text renders candidates as an aligned table with warning lines.
func Text(w io.Writer, cands []scan.Candidate) {
	if len(cands) == 0 {
		fmt.Fprintln(w, "No cleanup candidates found. ✨")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "REPO\tTYPE\tLAST PUSH\tSTARS\tSUGGESTED\tREASON")
	for _, c := range cands {
		typ := "repo"
		if c.Repo.IsFork {
			typ = "fork"
		}
		reason := strings.Join(c.Reasons, "; ")
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n",
			c.Repo.Name, typ, c.Repo.PushedAt.Format("2006-01-02"),
			c.Repo.Stars, c.Suggested, reason)
		for _, warn := range c.Warnings {
			fmt.Fprintf(tw, "\t\t\t\t\t⚠ %s\n", warn)
		}
	}
	tw.Flush()
}

// JSON writes candidates as indented JSON.
func JSON(w io.Writer, cands []scan.Candidate) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(cands)
}
