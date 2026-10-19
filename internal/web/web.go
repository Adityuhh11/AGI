// Package web opens web searches and links in the user's browser. The browser is the one set in the
// settings, not the Windows default: the launcher's owner may keep one browser for the system and use
// another, and "open this search" must land in the one they are looking at.
//
// A Firefox-based browser (Zen, Firefox, LibreWolf...) is asked to search with `-search`, which uses the
// engine chosen inside it; any other browser, or an explicit search_url, gets an address built from a
// template. Measured in docs/eval/2026-10-17-zen-command-line.md.
package web

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	"agi/internal/config"
)

// defaultSearch is the template used when the browser cannot search by itself and none is configured.
const defaultSearch = "https://duckduckgo.com/?q={q}"

// firefoxFamily are the browsers that take `-search <text>` on their command line.
var firefoxFamily = map[string]bool{"zen": true, "firefox": true, "librewolf": true, "waterfox": true, "floorp": true}

// Web opens searches and links.
type Web struct {
	cfg atomic.Pointer[webConfig] // replaced as a whole by Update, so a search never sees half a change

	// find locates a browser by name; start runs a program without waiting; shell opens an address with
	// the Windows default browser. Tests replace them.
	find  func(name string) (string, error)
	start func(exe string, args ...string) error
	shell func(address string) error
}

type webConfig struct {
	browser   string
	searchURL string
	links     []config.Quicklink
}

// New returns a Web for the configured browser (empty: the Windows default), search template (may be
// empty) and quicklinks.
func New(browser, searchURL string, links []config.Quicklink) *Web {
	w := &Web{find: appPath, start: start, shell: shellOpen}
	w.Update(browser, searchURL, links)
	return w
}

// Update replaces the browser, search template and quicklinks; it is safe to call while searches run.
func (w *Web) Update(browser, searchURL string, links []config.Quicklink) {
	w.cfg.Store(&webConfig{browser: strings.TrimSpace(browser), searchURL: searchURL, links: slices.Clone(links)})
}

// Search opens a web search for text.
func (w *Web) Search(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("nothing to search for")
	}
	c := w.cfg.Load()
	if c.browser != "" && c.searchURL == "" && firefoxFamily[browserName(c.browser)] {
		exe, err := w.executable(c.browser)
		if err != nil {
			return err
		}
		return w.start(exe, "-search", text)
	}
	template := c.searchURL
	if template == "" {
		template = defaultSearch
	}
	return w.Open(Expand(template, text))
}

// Open opens an address in the browser.
func (w *Web) Open(address string) error {
	browser := w.cfg.Load().browser
	if browser == "" {
		return w.shell(address)
	}
	exe, err := w.executable(browser)
	if err != nil {
		return err
	}
	return w.start(exe, address)
}

// executable is the browser's program: the configured path, or the program Windows registers under the name.
func (w *Web) executable(browser string) (string, error) {
	if filepath.IsAbs(browser) {
		return browser, nil
	}
	exe, err := w.find(browserName(browser))
	if err != nil {
		return "", fmt.Errorf("the browser %q is not installed (set browser in the settings to its path)", browser)
	}
	return exe, nil
}

// browserName is what a browser is called in the Firefox list and in Windows' registry: "Zen" and
// "zen.exe" and `C:\Program Files\Zen Browser\zen.exe` are all zen.
func browserName(browser string) string {
	name := strings.ToLower(filepath.Base(browser))
	return strings.TrimSuffix(name, ".exe")
}

// Quicklink recognises a trigger word followed by text ("gh rust lifetimes") and returns the link and the text.
func (w *Web) Quicklink(typed string) (config.Quicklink, string, bool) {
	trigger, rest, found := strings.Cut(strings.TrimSpace(typed), " ")
	rest = strings.TrimSpace(rest)
	if !found || rest == "" {
		return config.Quicklink{}, "", false
	}
	for _, link := range w.cfg.Load().links {
		if strings.EqualFold(link.Trigger, trigger) {
			return link, rest, true
		}
	}
	return config.Quicklink{}, "", false
}

// Find returns the quicklink with this trigger.
func (w *Web) Find(trigger string) (config.Quicklink, bool) {
	for _, link := range w.cfg.Load().links {
		if strings.EqualFold(link.Trigger, trigger) {
			return link, true
		}
	}
	return config.Quicklink{}, false
}

// Expand puts text, escaped for an address, where the template has {q}.
func Expand(template, text string) string {
	return strings.Replace(template, "{q}", url.QueryEscape(text), 1)
}

// start runs a program and lets it go: the launcher does not wait for a browser.
func start(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
