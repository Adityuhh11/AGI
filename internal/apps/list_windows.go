package apps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const createNoWindow = 0x08000000

// WatchedFolders are the folders Windows builds its Start Menu from (the ones that exist). A program
// installed or removed changes one of them, which is the cue to refresh the list.
func WatchedFolders() []string {
	var found []string
	for _, base := range []string{os.Getenv("ProgramData"), os.Getenv("APPDATA")} {
		if base == "" {
			continue
		}
		dir := filepath.Join(base, "Microsoft", "Windows", "Start Menu", "Programs")
		if _, err := os.Stat(dir); err == nil {
			found = append(found, dir)
		}
	}
	return found
}

// startApps asks Windows for everything its Start Menu shows. It takes over a second (mostly PowerShell
// starting), which is why it runs in the background and its answer is cached.
func startApps(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	script := "[Console]::OutputEncoding=[Text.Encoding]::UTF8; Get-StartApps | ConvertTo-Json -Compress"
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	// The launcher has no console; without this flag a console window would flash on every refresh.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run Get-StartApps: %w", err)
	}
	return out, nil
}

// list is the Start Menu as the catalog wants it.
func list(ctx context.Context) ([]App, error) {
	data, err := startApps(ctx)
	if err != nil {
		return nil, err
	}
	return parse(data)
}
