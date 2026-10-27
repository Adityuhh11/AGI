// Package autostart starts the launcher when its user signs in. It uses the per-user Run key, which
// needs no administrator rights and which Task Manager's Startup apps list shows and can switch off.
package autostart

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Variables only so the test can point them at a scratch key.
var (
	runKey      = `Software\Microsoft\Windows\CurrentVersion\Run`
	approvedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
)

const valueName = "Omni"

func command() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate the executable: %w", err)
	}
	return `"` + exe + `"`, nil
}

// Enabled reports whether signing in starts this very executable. A copy that was moved or replaced
// since the entry was written counts as off, so turning it on again repairs the path.
func Enabled() bool {
	want, err := command()
	if err != nil {
		return false
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	got, _, err := k.GetStringValue(valueName)
	return err == nil && strings.EqualFold(got, want) && !switchedOffInTaskManager()
}

// switchedOffInTaskManager reads the flag Task Manager keeps beside the Run key: an odd first byte
// means the user turned the entry off there.
func switchedOffInTaskManager() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, approvedKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	flag, _, err := k.GetBinaryValue(valueName)
	return err == nil && len(flag) > 0 && flag[0]%2 == 1
}

// Set turns starting at sign-in on or off. Turning it on also clears a Task Manager "disabled" flag,
// because the person asked for it here.
func Set(on bool) error {
	if !on {
		k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("open the Run key: %w", err)
		}
		defer k.Close()
		if err := k.DeleteValue(valueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("remove the Run entry: %w", err)
		}
		return nil
	}
	cmd, err := command()
	if err != nil {
		return err
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open the Run key: %w", err)
	}
	defer k.Close()
	if err := k.SetStringValue(valueName, cmd); err != nil {
		return fmt.Errorf("write the Run entry: %w", err)
	}
	if approved, err := registry.OpenKey(registry.CURRENT_USER, approvedKey, registry.SET_VALUE); err == nil {
		defer approved.Close()
		_ = approved.DeleteValue(valueName) // absent means enabled
	}
	return nil
}
