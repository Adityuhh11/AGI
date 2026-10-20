// Package update finds out whether a newer release of the launcher exists. It only asks; installing
// is the person's choice, from the release page. The one request goes to api.github.com and carries
// nothing about the user or the machine.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// latestURL answers with the newest release that is neither a draft nor a pre-release. A variable only so that a test
// build can point it at a local server (-ldflags "-X agi/internal/update.latestURL=..."); the release build never does.
var latestURL = "https://api.github.com/repos/Adityuhh11/AGI/releases/latest"

// Release is a published version: its number (without the "v"), its page, and the zip that can be installed
// from the program (empty when the release has none that can be verified: the page is all there is then).
type Release struct {
	Version string
	URL     string
	Asset   Asset
}

// Asset is the zip of a release: where it is, how big, and the SHA-256 GitHub published for it (64 lower-case hex digits).
type Asset struct {
	Name   string
	URL    string
	Size   int64
	SHA256 string
}

// assetOf picks the Windows zip of a release out of GitHub's list of its files. A zip whose digest is missing,
// is not SHA-256 or is not 64 hex digits is left out: nothing is installed that cannot be checked.
func assetOf(version string, files []githubFile) Asset {
	want := "agi-" + version + "-windows-x64.zip"
	for _, f := range files {
		if f.Name != want {
			continue
		}
		sum, ok := strings.CutPrefix(f.Digest, "sha256:")
		if raw, err := hex.DecodeString(sum); !ok || err != nil || len(raw) != sha256.Size {
			return Asset{}
		}
		return Asset{Name: f.Name, URL: f.URL, Size: f.Size, SHA256: strings.ToLower(sum)}
	}
	return Asset{}
}

// githubFile is a file of a release as GitHub's API lists it.
type githubFile struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

// Checker asks GitHub and remembers what the last check found.
type Checker struct {
	current string
	url     string
	client  *http.Client
	changed func()

	mu    sync.Mutex
	found *Release
}

// NewChecker returns a Checker for a build whose version is current. changed, if set, is called after
// every check that succeeded, from the goroutine that checked.
func NewChecker(current string, changed func()) *Checker {
	return &Checker{current: current, url: latestURL, client: &http.Client{Timeout: 10 * time.Second}, changed: changed}
}

// Run checks a minute after the launcher starts and then once a day, whenever enabled says so. A
// failed check is only logged: the next one tries again.
func (c *Checker) Run(ctx context.Context, enabled func() bool) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if enabled() {
			if _, _, err := c.Check(ctx); err != nil {
				log.Printf("update check: %v", err)
			}
		}
		timer.Reset(24 * time.Hour)
	}
}

// Check returns the newest release and whether it is newer than this build.
func (c *Checker) Check(ctx context.Context) (Release, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return Release{}, false, fmt.Errorf("build the request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "agi-launcher/"+c.current) // GitHub refuses requests without one
	resp, err := c.client.Do(req)
	if err != nil {
		return Release{}, false, fmt.Errorf("ask GitHub for the latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Release{}, false, fmt.Errorf("no release has been published yet")
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, false, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var body struct {
		Tag    string       `json:"tag_name"`
		URL    string       `json:"html_url"`
		Assets []githubFile `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return Release{}, false, fmt.Errorf("read GitHub's answer: %w", err)
	}
	rel := Release{Version: strings.TrimPrefix(body.Tag, "v"), URL: body.URL}
	rel.Asset = assetOf(rel.Version, body.Assets)
	newer := Newer(rel.Version, c.current)
	c.mu.Lock()
	c.found = nil
	if newer {
		c.found = &rel
	}
	c.mu.Unlock()
	if c.changed != nil {
		c.changed()
	}
	return rel, newer, nil
}

// Found is the newer release the last check saw, if there was one. It makes no request.
func (c *Checker) Found() (Release, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.found == nil {
		return Release{}, false
	}
	return *c.found, true
}

// Newer reports whether latest is a later version than current. Both are "1.2.3", with an optional
// "-suffix" that makes a version older than the same number without one. A build that has no version
// number (a test build named by date and commit) is older than any release; "dev" is a developer's
// build and is never offered an update.
func Newer(latest, current string) bool {
	if current == "dev" {
		return false
	}
	l, lPre, ok := parse(latest)
	if !ok {
		return false
	}
	c, cPre, ok := parse(current)
	if !ok {
		return true
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return cPre && !lPre
}

// parse reads "1.2.3" or "1.2.3-rc1" (a leading "v" is fine). The date-and-commit names of test
// builds ("2026.10.18-977d522") look like versions but are not: a first number from 2000 up is a year.
func parse(v string) (nums [3]int, pre, ok bool) {
	v = strings.TrimPrefix(v, "v")
	core, suffix, hasSuffix := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return nums, false, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nums, false, false
		}
		nums[i] = n
	}
	if nums[0] >= 2000 { // a year: 2026.10.18-<commit>
		return nums, false, false
	}
	return nums, hasSuffix && suffix != "", true
}
