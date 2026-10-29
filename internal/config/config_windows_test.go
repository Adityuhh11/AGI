package config

import "testing"

func TestDataDirOnANetworkShareIsRefused(t *testing.T) {
	cfg := Default()
	cfg.Search.DataDir = `\server\share\Omni`
	if err := cfg.Validate(); err == nil {
		t.Error("a network share was accepted for the engine's data")
	}
}

func TestExcludedUnderIgnoresTheCaseOfAWindowsPath(t *testing.T) {
	got := ExcludedUnder(`C:\Docs`, []string{`c:\docs\other`})
	if len(got) != 1 || got[0] != "other" {
		t.Fatalf("ExcludedUnder = %v: Windows paths do not care about case", got)
	}
}
