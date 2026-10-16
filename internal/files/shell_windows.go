package files

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// windowsShell is the real opener.
type windowsShell struct{}

func (windowsShell) Default(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

func (windowsShell) Editor(path string, line int) error {
	if code := visualStudioCode(); code != "" {
		target := path
		if line > 0 {
			target = fmt.Sprintf("%s:%d", path, line)
		}
		return start(exec.Command(code, "-g", target))
	}
	return start(exec.Command("notepad.exe", path))
}

func (windowsShell) Reveal(path string) error {
	return start(exec.Command("explorer.exe", "/select,"+path))
}

// start launches a program and lets it go: the launcher does not wait for editors.
func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// visualStudioCode returns the path of Code.exe if it is installed in one of its usual places.
func visualStudioCode() string {
	var candidates []string
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "Programs", "Microsoft VS Code", "Code.exe"))
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		if dir := os.Getenv(env); dir != "" {
			candidates = append(candidates, filepath.Join(dir, "Microsoft VS Code", "Code.exe"))
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}
