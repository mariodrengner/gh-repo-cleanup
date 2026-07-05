// Package github is a thin wrapper around the GitHub REST API via go-gh.
package github

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

type Client struct {
	rest *api.RESTClient
}

func New() (*Client, error) {
	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, err
	}
	return &Client{rest: rest}, nil
}

func NewWithOptions(opts api.ClientOptions) (*Client, error) {
	rest, err := api.NewRESTClient(opts)
	if err != nil {
		return nil, err
	}
	return &Client{rest: rest}, nil
}

type apiRepo struct {
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	Homepage      string    `json:"homepage"`
	Topics        []string  `json:"topics"`
	Fork          bool      `json:"fork"`
	Archived      bool      `json:"archived"`
	PushedAt      time.Time `json:"pushed_at"`
	Stars         int       `json:"stargazers_count"`
	Forks         int       `json:"forks_count"`
	OpenIssues    int       `json:"open_issues_count"`
	DefaultBranch string    `json:"default_branch"`
	Parent        *struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	} `json:"parent"`
}

func toRepo(r apiRepo) scan.Repo {
	out := scan.Repo{
		Name:          r.Name,
		NameWithOwner: r.FullName,
		Description:   r.Description,
		Homepage:      r.Homepage,
		Topics:        r.Topics,
		IsFork:        r.Fork,
		IsArchived:    r.Archived,
		PushedAt:      r.PushedAt,
		Stars:         r.Stars,
		Forks:         r.Forks,
		OpenIssues:    r.OpenIssues,
		DefaultBranch: r.DefaultBranch,
	}
	if r.Fork {
		out.AheadBy = scan.AheadUnknown
	}
	return out
}

func (c *Client) Username() (string, error) {
	var u struct {
		Login string `json:"login"`
	}
	err := c.get("user", &u)
	return u.Login, err
}

func (c *Client) ListRepos() ([]scan.Repo, error) {
	var out []scan.Repo
	for page := 1; ; page++ {
		var batch []apiRepo
		path := fmt.Sprintf("user/repos?affiliation=owner&per_page=100&page=%d", page)
		if err := c.get(path, &batch); err != nil {
			return nil, err
		}
		for _, r := range batch {
			out = append(out, toRepo(r))
		}
		if len(batch) < 100 {
			return out, nil
		}
	}
}

// EnrichFork fills Parent and AheadBy. A failed upstream comparison is
// non-fatal: AheadBy stays AheadUnknown so the scan can warn. If the repo
// itself is inaccessible (e.g. HTTP 451 legal block, 404 gone), enrichment is
// silently skipped so the scan can continue.
// HasOpenUpstreamPR is NOT set here; callers must apply the result of
// OpenPRTargets after enrichment.
func (c *Client) EnrichFork(login string, r *scan.Repo) error {
	var full apiRepo
	if err := c.get("repos/"+r.NameWithOwner, &full); err != nil {
		var httpErr *api.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode != 401 && httpErr.StatusCode < 500 {
			// Repo is inaccessible (legal block, gone, forbidden) — skip silently.
			return nil
		}
		return err
	}
	if full.Parent == nil {
		return nil // fork without reachable parent (deleted upstream)
	}
	r.Parent = full.Parent.FullName

	var cmp struct {
		AheadBy int `json:"ahead_by"`
	}
	comparePath := fmt.Sprintf("repos/%s/compare/%s...%s:%s?per_page=1",
		r.Parent, full.Parent.DefaultBranch, login, r.DefaultBranch)
	if err := c.get(comparePath, &cmp); err != nil {
		r.AheadBy = scan.AheadUnknown
	} else {
		r.AheadBy = cmp.AheadBy
	}
	return nil
}

// OpenPRTargets returns the set of "owner/repo" names that currently have
// open PRs authored by login. PRs against archived (read-only) repos are
// excluded, because they can never be merged and deleting the fork loses
// nothing actionable. One paginated search instead of one search per fork —
// the per-fork variant trips GitHub's secondary rate limit.
func (c *Client) OpenPRTargets(login string) (map[string]bool, error) {
	set := make(map[string]bool)
	q := url.QueryEscape("type:pr state:open archived:false author:" + login)
	for page := 1; ; page++ {
		var result struct {
			TotalCount int `json:"total_count"`
			Items      []struct {
				RepositoryURL string `json:"repository_url"`
			} `json:"items"`
		}
		path := fmt.Sprintf("search/issues?q=%s&per_page=100&page=%d", q, page)
		if err := c.get(path, &result); err != nil {
			return nil, err
		}
		for _, item := range result.Items {
			// repository_url = https://<host>/repos/OWNER/REPO
			if idx := strings.LastIndex(item.RepositoryURL, "/repos/"); idx >= 0 {
				set[item.RepositoryURL[idx+len("/repos/"):]] = true
			}
		}
		if len(result.Items) < 100 || len(set) >= result.TotalCount {
			return set, nil
		}
	}
}

func (c *Client) Archive(nameWithOwner string) error {
	body := bytes.NewReader([]byte(`{"archived":true}`))
	return c.rest.Patch("repos/"+nameWithOwner, body, nil)
}

func (c *Client) Delete(nameWithOwner string) error {
	return c.rest.Delete("repos/"+nameWithOwner, nil)
}

// HasDeleteScope inspects the token's OAuth scopes via the response header.
func (c *Client) HasDeleteScope() (bool, error) {
	resp, err := c.rest.Request("GET", "user", nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	scopes := resp.Header.Get("X-Oauth-Scopes")
	for _, s := range strings.Split(scopes, ",") {
		if strings.TrimSpace(s) == "delete_repo" {
			return true, nil
		}
	}
	return false, nil
}

// get wraps GET with one retry on rate-limit or server errors.
func (c *Client) get(path string, out any) error {
	err := c.rest.Get(path, out)
	if err == nil {
		return nil
	}
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) &&
		(httpErr.StatusCode == 403 || httpErr.StatusCode == 429 || httpErr.StatusCode >= 500) {
		time.Sleep(2 * time.Second)
		return c.rest.Get(path, out)
	}
	return err
}
