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
