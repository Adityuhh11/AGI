package web

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// browserApps are the browsers worth offering by name in the settings, and the app each is on a Mac. The names are the ones the
// settings and the Firefox list use (browserName).
var browserApps = map[string]string{
	"safari": "Safari", "chrome": "Google Chrome", "msedge": "Microsoft Edge", "firefox": "Firefox", "brave": "Brave Browser",
	"arc": "Arc", "zen": "Zen", "vivaldi": "Vivaldi", "opera": "Opera", "librewolf": "LibreWolf", "waterfox": "Waterfox", "floorp": "Floorp",
}

// knownBrowsers lists them in the order the settings show them.
var knownBrowsers = []string{"safari", "chrome", "msedge", "firefox", "arc", "brave", "zen", "vivaldi", "opera", "librewolf", "waterfox", "floorp"}

// appPath finds the bundle of a known browser in /Applications or ~/Applications.
func appPath(name string) (string, error) {
	app, ok := browserApps[name]
	if !ok {
		return "", errors.New("not a browser this knows")
	}
	folders := []string{"/Applications"}
	if home, err := os.UserHomeDir(); err == nil {
		folders = append(folders, filepath.Join(home, "Applications"))
	}
	for _, folder := range folders {
		bundle := filepath.Join(folder, app+".app")
		if _, err := os.Stat(bundle); err == nil {
			return bundle, nil
		}
	}
	return "", errors.New("not installed")
}

// shellOpen opens an address with the default browser.
func shellOpen(address string) error { return start("open", address) }

// Installed returns the known browsers that are installed on this machine.
func Installed() []string {
	var found []string
	for _, name := range knownBrowsers {
		if _, err := appPath(name); err == nil {
			found = append(found, name)
		}
	}
	return found
}

// searchesByCommandLine is false on a Mac: `open --args` hands its arguments to a browser only when it starts, so a Firefox-based
// browser that is already open would ignore `-search`. It searches with the address template instead.
const searchesByCommandLine = false

// start opens an address in a browser. A bundle is opened with `open -a`, which raises the browser if it runs and starts it if not;
// anything else (a program the person named by path) is run as it is.
func start(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	if strings.HasSuffix(exe, ".app") {
		cmd = exec.Command("open", append([]string{"-a", exe}, args...)...)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
