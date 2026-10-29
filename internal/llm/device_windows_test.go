package llm

import "testing"

// fakeListing is what `llama-server --list-devices` prints on the machine this was written for, and onTheGraphicsCard what the
// server is then started with.
const fakeListing = "  Vulkan1: NVIDIA GeForce RTX 4050 Laptop GPU (5920 MiB, 5152 MiB free)\n"

var onTheGraphicsCard = []string{"-ngl", "99", "Vulkan1"}

func TestChooseDevicePicksTheDiscreteCardByNameNotByMemory(t *testing.T) {
	for name, tc := range map[string]struct{ listing, want string }{
		"intel and nvidia": {"Available devices:\n  Vulkan0: Intel(R) UHD Graphics (8038 MiB, 7380 MiB free)\n  Vulkan1: NVIDIA GeForce RTX 4050 Laptop GPU (5920 MiB, 5152 MiB free)\n", "Vulkan1"},
		"crlf":             {"  Vulkan0: Intel(R) UHD Graphics (8038 MiB, 7380 MiB free)\r\n  Vulkan1: NVIDIA GeForce RTX 3060 (12000 MiB, 9000 MiB free)\r\n", "Vulkan1"},
		"amd over unknown": {"  Vulkan0: Mystery GPU (2000 MiB, 1000 MiB free)\n  Vulkan1: AMD Radeon RX 6600 (8176 MiB, 8000 MiB free)\n", "Vulkan1"},
		"only integrated":  {"  Vulkan0: Intel(R) Iris(R) Xe Graphics (16000 MiB, 12000 MiB free)\n", ""},
		"unknown card":     {"  Vulkan0: Some New GPU (4000 MiB, 3000 MiB free)\n", "Vulkan0"},
		"no devices":       {"Available devices:\n", ""},
		"empty":            {"", ""},
	} {
		if got := ChooseDevice(tc.listing); got != tc.want {
			t.Errorf("%s: ChooseDevice = %q, want %q", name, got, tc.want)
		}
	}
}
