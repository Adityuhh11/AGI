package llm

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"syscall"
	"time"
)

// "  Vulkan1: NVIDIA GeForce RTX 4050 Laptop GPU (5920 MiB, 5152 MiB free)"
var deviceLine = regexp.MustCompile(`^\s*([A-Za-z]+\d+):\s*(.+?)\s*\(\d+ MiB`)

var (
	discrete   = []*regexp.Regexp{regexp.MustCompile(`(?i)nvidia|geforce|quadro|rtx`), regexp.MustCompile(`(?i)amd|radeon`)}
	integrated = regexp.MustCompile(`(?i)intel.*(uhd|iris|hd graphics|xe graphics)`)
)

// ChooseDevice returns the id ("Vulkan1") of the graphics card to use from the output of `llama-server --list-devices`, or ""
// for the processor. It picks by name, never by the largest memory: an integrated GPU reports the computer's shared memory and
// would win. The rule is the file-search engine's (sidecar/src/omni_memory/embedding/llama_server.py, choose_device).
func ChooseDevice(listing string) string {
	type device struct{ id, name string }
	var devices []device
	for _, line := range regexp.MustCompile(`\r?\n`).Split(listing, -1) {
		if m := deviceLine.FindStringSubmatch(line); m != nil {
			devices = append(devices, device{m[1], m[2]})
		}
	}
	for _, pattern := range discrete {
		for _, d := range devices {
			if pattern.MatchString(d.name) {
				return d.id
			}
		}
	}
	for _, d := range devices {
		if !integrated.MatchString(d.name) {
			return d.id
		}
	}
	return ""
}

// listDevices runs the server's device listing; it prints to stdout and stderr both.
func listDevices(ctx context.Context, exe string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "--list-devices")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("list the graphics devices: %w", err)
	}
	return string(out), nil
}

// chooseDevice is the device to try first: the discrete card by name. "" is the processor.
func chooseDevice(listing string) string { return ChooseDevice(listing) }

// gpuArgs puts the whole model on the device with this id.
func gpuArgs(id string) []string { return []string{"-ngl", "99", "--device", id} }
