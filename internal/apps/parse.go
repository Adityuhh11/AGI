package apps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// startApp is one object of `Get-StartApps | ConvertTo-Json`.
type startApp struct {
	Name  string `json:"Name"`
	AppID string `json:"AppID"`
}

// parse reads the JSON that `Get-StartApps | ConvertTo-Json` prints (an array, or a bare object when
// there is exactly one entry) and keeps the entries that are applications.
func parse(data []byte) ([]App, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // a byte order mark, which PowerShell may print
	var entries []startApp
	if err := json.Unmarshal(data, &entries); err != nil {
		var one startApp
		if err2 := json.Unmarshal(data, &one); err2 != nil {
			return nil, fmt.Errorf("read the app list: %w", err)
		}
		entries = []startApp{one}
	}
	seen := map[string]bool{}
	var found []App
	for _, e := range entries {
		app := App{ID: e.AppID, Name: strings.TrimSpace(e.Name)}
		if keep(app) && !seen[app.ID] {
			seen[app.ID] = true
			found = append(found, app)
		}
	}
	return found, nil
}

// launchable are the kinds of file a Start Menu entry may point at and still be something to run.
var launchable = map[string]bool{".exe": true, ".msc": true, ".bat": true, ".cmd": true}

// keep says whether a Start Menu entry is an application. The list holds a lot more than programs:
// documentation (.html, .pdf, .chm), websites, release notes, folders and the odd stray download. On the
// machine this was written on, 203 entries held 60 Store apps, 77 desktop programs with plain ids,
// 49 executables, 3 admin consoles and about 15 entries that are not applications.
func keep(a App) bool {
	if a.Name == "" || a.ID == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(a.Name), "uninstall ") {
		return false
	}
	id := strings.ToLower(a.ID)
	for _, web := range []string{"http://", "https://", "file://"} {
		if strings.HasPrefix(id, web) {
			return false
		}
	}
	if !strings.Contains(id, `\`) {
		return true // an AppUserModelID, a Store app, or a launcher URL such as steam://rungameid/...
	}
	dot := strings.LastIndex(id, ".")
	return dot > strings.LastIndex(id, `\`) && launchable[id[dot:]]
}
