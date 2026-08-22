// Package check verifies that every manifest package name still resolves
// upstream.
//
// This is the guard against the catalogue rotting. Two things go wrong over
// time: a name is transcribed wrongly when the entry is authored, and a name
// that was right stops being right because upstream renamed it or moved it
// between formula and cask. Both fail identically for the user — an install
// that dies — and neither is caught by `validate`, which is deliberately
// offline and only checks internal consistency.
//
// Unlike validate, this makes network calls and is meant for CI on a schedule.
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Verdict classifies one tool's check.
type Verdict int

const (
	// VerdictOK means the package resolves as declared.
	VerdictOK Verdict = iota

	// VerdictMissing means nothing upstream answers to that name.
	VerdictMissing

	// VerdictWrongType means the name exists, but as the other Homebrew
	// kind — a formula declared as a cask, or the reverse. This is the
	// mitmproxy case: it really was a formula and is now a cask, and the
	// manifest was silently wrong until someone tried to install it.
	VerdictWrongType

	// VerdictDeprecated means it resolves and still installs, but upstream
	// has flagged it. Worth knowing and worth acting on eventually, but not
	// broken today — reported as a warning so a red build still means
	// "something is actually broken".
	VerdictDeprecated

	// VerdictDisabled means upstream has withdrawn it. It will not install.
	VerdictDisabled

	// VerdictSkipped means the backend has no checker yet.
	VerdictSkipped

	// VerdictError means the check itself failed — network, rate limit —
	// which is not the same as the package being wrong and must never be
	// reported as a catalogue problem.
	VerdictError
)

func (v Verdict) String() string {
	switch v {
	case VerdictOK:
		return "ok"
	case VerdictMissing:
		return "missing"
	case VerdictWrongType:
		return "wrong-type"
	case VerdictDeprecated:
		return "deprecated"
	case VerdictDisabled:
		return "disabled"
	case VerdictSkipped:
		return "skipped"
	case VerdictError:
		return "error"
	}
	return "unknown"
}

// Bad reports whether this verdict means the catalogue is broken now.
func (v Verdict) Bad() bool {
	return v == VerdictMissing || v == VerdictWrongType || v == VerdictDisabled
}

// Warning reports a verdict worth surfacing that is not yet a failure.
func (v Verdict) Warning() bool { return v == VerdictDeprecated }

// Result is one tool's outcome.
type Result struct {
	Tool    *manifest.Tool
	Verdict Verdict

	// Detail explains the verdict.
	Detail string

	// Suggestion is the manifest change that would fix it, when known.
	Suggestion string
}

// Report is a whole run.
type Report struct {
	Results []Result
}

// Problems returns only the results that need fixing.
func (r *Report) Problems() []Result {
	var out []Result
	for _, res := range r.Results {
		if res.Verdict.Bad() {
			out = append(out, res)
		}
	}
	return out
}

// Warnings returns results worth surfacing that are not failures.
func (r *Report) Warnings() []Result {
	var out []Result
	for _, res := range r.Results {
		if res.Verdict.Warning() {
			out = append(out, res)
		}
	}
	return out
}

// Errors returns checks that could not be completed. Reported separately from
// problems: a flaky network must not look like a broken catalogue.
func (r *Report) Errors() []Result {
	var out []Result
	for _, res := range r.Results {
		if res.Verdict == VerdictError {
			out = append(out, res)
		}
	}
	return out
}

// Counts summarises the run.
func (r *Report) Counts() map[Verdict]int {
	m := map[Verdict]int{}
	for _, res := range r.Results {
		m[res.Verdict]++
	}
	return m
}

// Checker verifies one backend's package names.
type Checker interface {
	Backend() manifest.Backend
	Check(ctx context.Context, t *manifest.Tool) Result
}

// Options configures a run.
type Options struct {
	// Concurrency caps parallel requests. Kept modest on purpose: this
	// hammers someone else's API and politeness costs almost nothing when
	// the whole run takes seconds either way.
	Concurrency int

	// Timeout applies to each individual request.
	Timeout time.Duration

	// Progress is called as each tool completes.
	Progress func(done, total int, r Result)
}

func (o *Options) applyDefaults() {
	if o.Concurrency <= 0 {
		o.Concurrency = 8
	}
	if o.Timeout <= 0 {
		o.Timeout = 20 * time.Second
	}
}

// Run checks every tool in the catalogue.
func Run(ctx context.Context, cat *manifest.Catalogue, opts Options) (*Report, error) {
	opts.applyDefaults()

	client := &http.Client{Timeout: opts.Timeout}
	checkers := map[manifest.Backend]Checker{
		manifest.BackendBrew: &brewChecker{client: client, cask: false},
		manifest.BackendCask: &brewChecker{client: client, cask: true},
	}

	rep := &Report{Results: make([]Result, len(cat.Tools))}

	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0

	for i, t := range cat.Tools {
		wg.Add(1)
		go func(i int, t *manifest.Tool) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var res Result
			if c, ok := checkers[t.Backend]; ok {
				res = c.Check(ctx, t)
			} else {
				res = Result{Tool: t, Verdict: VerdictSkipped,
					Detail: fmt.Sprintf("no checker for backend %q yet", t.Backend)}
			}
			rep.Results[i] = res

			mu.Lock()
			done++
			if opts.Progress != nil {
				opts.Progress(done, len(cat.Tools), res)
			}
			mu.Unlock()
		}(i, t)
	}
	wg.Wait()

	sort.SliceStable(rep.Results, func(a, b int) bool {
		return rep.Results[a].Tool.ID < rep.Results[b].Tool.ID
	})
	return rep, nil
}

// brewChecker verifies formula and cask names against Homebrew's API.
type brewChecker struct {
	client *http.Client
	cask   bool
}

func (c *brewChecker) Backend() manifest.Backend {
	if c.cask {
		return manifest.BackendCask
	}
	return manifest.BackendBrew
}

const apiBase = "https://formulae.brew.sh/api"

// brewInfo is the small slice of the API response worth reading.
//
// Deliberately omits name and token: formulae return "name" as a string while
// casks return it as an array, so a single struct cannot decode both, and
// nothing here needs it. Existence is answered by the status code.
type brewInfo struct {
	Deprecated        bool   `json:"deprecated"`
	DeprecationReason string `json:"deprecation_reason"`
	Disabled          bool   `json:"disabled"`
	DisableReason     string `json:"disable_reason"`
}

func (c *brewChecker) Check(ctx context.Context, t *manifest.Tool) Result {
	kind := "formula"
	other := "cask"
	if c.cask {
		kind, other = "cask", "formula"
	}

	info, status, err := c.fetch(ctx, kind, t.Package)
	if err != nil {
		return Result{Tool: t, Verdict: VerdictError, Detail: err.Error()}
	}

	if status == http.StatusNotFound {
		// The name may exist as the other kind. That is a far more
		// useful diagnosis than "missing", and it is the single most
		// common way a Homebrew entry goes stale.
		if _, otherStatus, err := c.fetch(ctx, other, t.Package); err == nil && otherStatus == http.StatusOK {
			return Result{
				Tool:    t,
				Verdict: VerdictWrongType,
				Detail:  fmt.Sprintf("%q is a %s, not a %s", t.Package, other, kind),
				Suggestion: fmt.Sprintf("change backend to %q in [tool.macos]",
					otherBackend(c.cask)),
			}
		}
		return Result{Tool: t, Verdict: VerdictMissing,
			Detail: fmt.Sprintf("no %s named %q", kind, t.Package)}
	}

	if status != http.StatusOK {
		return Result{Tool: t, Verdict: VerdictError,
			Detail: fmt.Sprintf("unexpected status %d", status)}
	}

	switch {
	case info.Disabled:
		return Result{Tool: t, Verdict: VerdictDisabled,
			Detail: "withdrawn upstream: " + fallback(info.DisableReason, "no reason given")}
	case info.Deprecated:
		return Result{Tool: t, Verdict: VerdictDeprecated,
			Detail: "deprecated upstream: " + fallback(info.DeprecationReason, "no reason given")}
	}

	return Result{Tool: t, Verdict: VerdictOK, Detail: kind + " " + t.Package}
}

func (c *brewChecker) fetch(ctx context.Context, kind, name string) (*brewInfo, int, error) {
	url := fmt.Sprintf("%s/%s/%s.json", apiBase, kind, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "macwtf-check/0.1 (+https://github.com/naturalstate/macWTF)")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, resp.StatusCode, nil
	}

	var info brewInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("decode %s response: %w", kind, err)
	}
	return &info, resp.StatusCode, nil
}

func otherBackend(wasCask bool) manifest.Backend {
	if wasCask {
		return manifest.BackendBrew
	}
	return manifest.BackendCask
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
