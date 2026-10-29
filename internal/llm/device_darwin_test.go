package llm

import (
	"slices"
	"testing"
)

// fakeListing is empty: a Mac is not asked which devices it has. onTheGraphicsCard is what the server is started with.
const fakeListing = ""

var onTheGraphicsCard = []string{"-ngl", "99"}

func TestAMacAlwaysTriesMetalAndNeverNamesADevice(t *testing.T) {
	if chooseDevice("") != metal || chooseDevice("anything") != metal {
		t.Fatal("a Mac's GPU is Metal whatever the server lists")
	}
	if args := gpuArgs(metal); !slices.Equal(args, []string{"-ngl", "99"}) {
		t.Fatalf("gpuArgs = %v: --device would name a device the Mac's server does not list", args)
	}
}
