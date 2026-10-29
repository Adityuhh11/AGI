package config

import (
	"path/filepath"
	"testing"
)

func TestDataAndLogFoldersAreCalledOmni(t *testing.T) {
	data, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(data) != "Omni" {
		t.Errorf("DataDir = %q, want a folder called Omni (removing the data refuses any other name)", data)
	}
	logs, err := LogDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(logs) != "logs" && filepath.Base(logs) != "Omni" {
		t.Errorf("LogDir = %q, want .../Omni/logs or .../Logs/Omni", logs)
	}
}
