package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/mariodrengner/gh-repo-cleanup/internal/scan"
)

// rewriteTransport sends every request to the test server, regardless of host.
type rewriteTransport struct{ base *url.URL }

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.base.Scheme
	req.URL.Host = t.base.Host
	return http.DefaultTransport.RoundTrip(req)
}

func testClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	c, err := NewWithOptions(api.ClientOptions{
		Host:         "github.com",
		AuthToken:    "test-token",
		Transport:    rewriteTransport{base: u},
		LogIgnoreEnv: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestUsername(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"login":"mario"}`)
	}))
	got, err := c.Username()
	if err != nil || got != "mario" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestGetRetriesOnceOnRateLimit(t *testing.T) {
	var requestCount int
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"message":"rate limited"}`)
			return
		}
		// Second request succeeds
		fmt.Fprint(w, `{"login":"mario"}`)
	}))
	got, err := c.Username()
	if err != nil {
		t.Fatalf("Username() failed: %v", err)
	}
	if got != "mario" {
		t.Fatalf("Username() got %q, want mario", got)
	}
	if requestCount != 2 {
		t.Fatalf("got %d requests, want 2", requestCount)
	}
}

func TestListReposPaginatesAndMapsFields(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if got := r.URL.Query().Get("affiliation"); got != "owner" {
			t.Errorf("affiliation = %q, want owner", got)
		}
		if page == "1" {
			repos := make([]map[string]any, 100)
			for i := range repos {
				repos[i] = map[string]any{"name": fmt.Sprintf("r%03d", i)}
			}
			json.NewEncoder(w).Encode(repos)
			return
		}
		fmt.Fprint(w, `[{"name":"last","full_name":"mario/last","fork":true,"archived":true,
			"stargazers_count":7,"forks_count":2,"open_issues_count":3,
			"default_branch":"main","pushed_at":"2024-01-02T03:04:05Z",
			"description":"desc","homepage":"https://x.y","topics":["a","b"]}]`)
	}))
	repos, err := c.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 101 {
		t.Fatalf("want 101 repos across pages, got %d", len(repos))
	}
	last := repos[100]
	if !last.IsFork || !last.IsArchived || last.Stars != 7 || last.Forks != 2 ||
		last.OpenIssues != 3 || last.NameWithOwner != "mario/last" ||
		last.DefaultBranch != "main" || last.Description != "desc" ||
		len(last.Topics) != 2 || last.PushedAt.Year() != 2024 {
		t.Fatalf("field mapping wrong: %+v", last)
	}
	if last.AheadBy != scan.AheadUnknown {
		t.Fatalf("fork AheadBy must default to AheadUnknown, got %d", last.AheadBy)
	}
}

func TestEnrichFork(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/mario/f":
			fmt.Fprint(w, `{"parent":{"full_name":"up/f","default_branch":"main"}}`)
		case strings.HasPrefix(r.URL.Path, "/repos/up/f/compare/"):
			fmt.Fprint(w, `{"ahead_by":3}`)
		case r.URL.Path == "/search/issues":
			fmt.Fprint(w, `{"total_count":1}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	repo := scan.Repo{Name: "f", NameWithOwner: "mario/f", IsFork: true,
		DefaultBranch: "main", AheadBy: scan.AheadUnknown}
	if err := c.EnrichFork("mario", &repo); err != nil {
		t.Fatal(err)
	}
	if repo.Parent != "up/f" || repo.AheadBy != 3 || !repo.HasOpenUpstreamPR {
		t.Fatalf("enrichment wrong: %+v", repo)
	}
}

func TestEnrichForkComparisonFailureIsNonFatal(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/mario/f":
			fmt.Fprint(w, `{"parent":{"full_name":"up/f","default_branch":"main"}}`)
		case strings.HasPrefix(r.URL.Path, "/repos/up/f/compare/"):
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Not Found"}`)
		case r.URL.Path == "/search/issues":
			fmt.Fprint(w, `{"total_count":0}`)
		}
	}))
	repo := scan.Repo{Name: "f", NameWithOwner: "mario/f", IsFork: true,
		DefaultBranch: "main", AheadBy: 5}
	if err := c.EnrichFork("mario", &repo); err != nil {
		t.Fatal(err)
	}
	if repo.AheadBy != scan.AheadUnknown {
		t.Fatalf("failed comparison must set AheadUnknown, got %d", repo.AheadBy)
	}
}

func TestArchiveSendsPatch(t *testing.T) {
	var method, path, body string
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		fmt.Fprint(w, `{}`)
	}))
	if err := c.Archive("mario/old"); err != nil {
		t.Fatal(err)
	}
	if method != "PATCH" || path != "/repos/mario/old" || !strings.Contains(body, `"archived":true`) {
		t.Fatalf("got %s %s body=%s", method, path, body)
	}
}

func TestDeleteSendsDelete(t *testing.T) {
	var method, path string
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	if err := c.Delete("mario/old"); err != nil {
		t.Fatal(err)
	}
	if method != "DELETE" || path != "/repos/mario/old" {
		t.Fatalf("got %s %s", method, path)
	}
}

func TestHasDeleteScope(t *testing.T) {
	for header, want := range map[string]bool{
		"gist, repo, delete_repo": true,
		"gist, repo":              false,
	} {
		c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Oauth-Scopes", header)
			fmt.Fprint(w, `{"login":"mario"}`)
		}))
		got, err := c.HasDeleteScope()
		if err != nil || got != want {
			t.Fatalf("scopes %q: got %v, %v; want %v", header, got, err, want)
		}
	}
}
