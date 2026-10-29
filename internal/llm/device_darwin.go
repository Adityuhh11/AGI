package llm

import "context"

// metal is the one graphics device of an Apple Silicon Mac, and it is the Mac's own memory (unified), so there is no card to pick
// by name and no video memory to fit into. The server is not asked which devices it has; it is started with every layer on the GPU and,
// when that fails (a virtual machine without one), on the processor.
const metal = "metal"

// listDevices has nothing to ask on a Mac.
func listDevices(context.Context, string) (string, error) { return "", nil }

// chooseDevice is always Metal.
func chooseDevice(string) string { return metal }

// gpuArgs puts the whole model on the GPU; with no --device the server uses the only one it has.
func gpuArgs(string) []string { return []string{"-ngl", "99"} }
