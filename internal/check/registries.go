package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/manifest"
)

// Checkers for the language registries.
//
// Homebrew was only ever half the catalogue. Roughly a hundred entries install
// through pipx, go, cargo or npm, and every one of them was reported as
// "skipped — no checker", which meant an unverifiable name stayed unverified
// forever and its tools stayed out of every profile.
//
// Each registry answers the same question — does this name resolve — and
// returns the upstream description while it is there, since that is a second
// request nobody should have to make.

// pypiChecker verifies Python package names.
type pypiChecker struct{ client *http.Client }

func (c *pypiChecker) Backend() manifest.Backend { return manifest.BackendPipx }

func (c *pypiChecker) Check(ctx context.Context, t *manifest.Tool) Result {
	// A VCS spec is not a registry name. pipx installs it directly from the
	// repository, so PyPI has nothing to say about it, and reporting it as
	// missing would be wrong.
	if isVCSSpec(t.Package) {
		return Result{Tool: t, Verdict: VerdictOK,
			Detail: "installs from a repository, not from PyPI"}
	}

	var doc struct {
		Info struct {
			Summary string `json:"summary"`
			Yanked  bool   `json:"yanked"`
		} `json:"info"`
	}
	status, err := getJSON(ctx, c.client, "https://pypi.org/pypi/"+t.Package+"/json", &doc)
	switch {
	case err != nil:
		return Result{Tool: t, Verdict: VerdictError, Detail: err.Error()}
	case status == http.StatusNotFound:
		return Result{Tool: t, Verdict: VerdictMissing,
			Detail: fmt.Sprintf("no PyPI package named %q", t.Package)}
	case status != http.StatusOK:
		return Result{Tool: t, Verdict: VerdictError,
			Detail: fmt.Sprintf("unexpected status %d", status)}
	}
	return Result{Tool: t, Verdict: VerdictOK,
		Detail: "pypi " + t.Package, Description: doc.Info.Summary}
}

// goChecker verifies Go module paths against the module proxy.
type goChecker struct{ client *http.Client }

func (c *goChecker) Backend() manifest.Backend { return manifest.BackendGo }

func (c *goChecker) Check(ctx context.Context, t *manifest.Tool) Result {
	mod := t.Package
	if i := strings.Index(mod, "@"); i >= 0 {
		mod = mod[:i]
	}

	// A command lives in a subdirectory of the module, and the proxy only
	// knows modules, so walk up until something answers. Without this every
	// .../cmd/subfinder path would report as missing.
	candidates := []string{mod}
	for p := mod; strings.Contains(p, "/"); {
		p = p[:strings.LastIndex(p, "/")]
		if strings.Count(p, "/") < 2 {
			break // shorter than host/owner/repo
		}
		candidates = append(candidates, p)
	}

	var lastStatus int
	for _, cand := range candidates {
		status, err := getRaw(ctx, c.client,
			"https://proxy.golang.org/"+strings.ToLower(cand)+"/@v/list")
		if err != nil {
			return Result{Tool: t, Verdict: VerdictError, Detail: err.Error()}
		}
		lastStatus = status
		if status == http.StatusOK {
			detail := "go module " + cand
			if cand != mod {
				detail = fmt.Sprintf("command in module %s", cand)
			}
			return Result{Tool: t, Verdict: VerdictOK, Detail: detail}
		}
	}
	if lastStatus == http.StatusNotFound || lastStatus == http.StatusGone {
		return Result{Tool: t, Verdict: VerdictMissing,
			Detail: fmt.Sprintf("no Go module for %q", mod)}
	}
	return Result{Tool: t, Verdict: VerdictError,
		Detail: fmt.Sprintf("unexpected status %d", lastStatus)}
}

// cratesChecker verifies crate names.
type cratesChecker struct{ client *http.Client }

func (c *cratesChecker) Backend() manifest.Backend { return manifest.BackendCargo }

func (c *cratesChecker) Check(ctx context.Context, t *manifest.Tool) Result {
	var doc struct {
		Crate struct {
			Description string `json:"description"`
		} `json:"crate"`
	}
	status, err := getJSON(ctx, c.client, "https://crates.io/api/v1/crates/"+t.Package, &doc)
	switch {
	case err != nil:
		return Result{Tool: t, Verdict: VerdictError, Detail: err.Error()}
	case status == http.StatusNotFound:
		return Result{Tool: t, Verdict: VerdictMissing,
			Detail: fmt.Sprintf("no crate named %q", t.Package)}
	case status != http.StatusOK:
		return Result{Tool: t, Verdict: VerdictError,
			Detail: fmt.Sprintf("unexpected status %d", status)}
	}
	return Result{Tool: t, Verdict: VerdictOK,
		Detail: "crate " + t.Package, Description: doc.Crate.Description}
}

// npmChecker verifies npm package names.
type npmChecker struct{ client *http.Client }

func (c *npmChecker) Backend() manifest.Backend { return manifest.BackendNPM }

func (c *npmChecker) Check(ctx context.Context, t *manifest.Tool) Result {
	var doc struct {
		Description string `json:"description"`
	}
	status, err := getJSON(ctx, c.client, "https://registry.npmjs.org/"+t.Package, &doc)
	switch {
	case err != nil:
		return Result{Tool: t, Verdict: VerdictError, Detail: err.Error()}
	case status == http.StatusNotFound:
		return Result{Tool: t, Verdict: VerdictMissing,
			Detail: fmt.Sprintf("no npm package named %q", t.Package)}
	case status != http.StatusOK:
		return Result{Tool: t, Verdict: VerdictError,
			Detail: fmt.Sprintf("unexpected status %d", status)}
	}
	return Result{Tool: t, Verdict: VerdictOK,
		Detail: "npm " + t.Package, Description: doc.Description}
}

// gitChecker verifies that a repository URL responds.
type gitChecker struct{ client *http.Client }

func (c *gitChecker) Backend() manifest.Backend { return manifest.BackendGit }

func (c *gitChecker) Check(ctx context.Context, t *manifest.Tool) Result {
	url := strings.TrimSuffix(t.Package, ".git")
	status, err := getRaw(ctx, c.client, url)
	switch {
	case err != nil:
		return Result{Tool: t, Verdict: VerdictError, Detail: err.Error()}
	case status == http.StatusNotFound:
		return Result{Tool: t, Verdict: VerdictMissing,
			Detail: "repository not found: " + t.Package}
	case status >= 400:
		return Result{Tool: t, Verdict: VerdictError,
			Detail: fmt.Sprintf("unexpected status %d", status)}
	}
	return Result{Tool: t, Verdict: VerdictOK, Detail: "repository reachable"}
}

// isVCSSpec reports whether a package is installed from a repository rather
// than from a registry.
func isVCSSpec(pkg string) bool {
	return strings.HasPrefix(pkg, "git+") ||
		strings.HasPrefix(pkg, "https://") ||
		strings.HasPrefix(pkg, "http://") ||
		strings.HasPrefix(pkg, "git@")
}

func newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "macwtf-check/0.1 (+https://github.com/naturalstate/macWTF)")
	return req, nil
}

// getJSON fetches and decodes, returning the status so 404 can be handled as
// an answer rather than an error.
func getJSON(ctx context.Context, c *http.Client, url string, into any) (int, error) {
	req, err := newRequest(ctx, url)
	if err != nil {
		return 0, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return resp.StatusCode, fmt.Errorf("decode response: %w", err)
	}
	return resp.StatusCode, nil
}

// getRaw fetches and discards the body, for existence checks.
func getRaw(ctx context.Context, c *http.Client, url string) (int, error) {
	req, err := newRequest(ctx, url)
	if err != nil {
		return 0, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

var _ = backend.BinaryName // keep the dependency explicit for future use
