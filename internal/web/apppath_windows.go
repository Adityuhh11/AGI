package web

import (
	"errors"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// appPath finds a program by the name Windows registers it under (`zen` is `zen.exe` under App Paths),
// looking at the current user first and then the machine.
func appPath(name string) (string, error) {
	key := `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\` + name + ".exe"
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(root, key, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		path, _, err := k.GetStringValue("")
		k.Close()
		if err == nil && path != "" {
			return path, nil
		}
	}
	return "", errors.New("not registered")
}

// shellOpen opens an address with the Windows default program for it (the default browser).
func shellOpen(address string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(address)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}

// knownBrowsers are the browsers worth offering by name in the settings; the names are the ones
// Windows registers them under (`msedge` is `msedge.exe` under App Paths).
var knownBrowsers = []string{"chrome", "msedge", "firefox", "zen", "brave", "vivaldi", "opera", "librewolf", "waterfox", "floorp"}

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
