package config

import "testing"

func TestDataDirOnANetworkShareIsRefused(t *testing.T) {
	cfg := Default()
	cfg.Search.DataDir = `\server\share\Omni`
	if err := cfg.Validate(); err == nil {
		t.Error("a network share was accepted for the engine's data")
	}
}
