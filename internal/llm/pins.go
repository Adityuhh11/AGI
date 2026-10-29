package llm

import "omni/internal/enginesetup"

// The pinned downloads that are the same on every system (Llama is per system: pins_windows.go, pins_darwin.go), measured in docs/eval/2026-10-28-chat-model.md. The model is Liquid AI's quantization-aware 4-bit
// build of LFM3-1.7B; it needs a llama.cpp build of b11960 or newer.
var (
	Model = enginesetup.Artifact{
		Name:   "LFM3-1.7B (QAD Q4_0)",
		URL:    "https://huggingface.co/LiquidAI/LFM3-1.7B-GGUF/resolve/4b423c332fd97961416f2e8b69467758fd27370d/LFM3-1.7B-QAD-Q4_0.gguf",
		Size:   1049615680,
		SHA256: "43a7d09a0e9cba91b63eb7052488b9564569bddfd65754d2e426465e572a7452",
	}
)

// Hosts are the servers turning Omni AI on talks to, for the settings page to show before the person agrees.
var Hosts = []string{"github.com", "huggingface.co"}

// LicenseURL is where the model's licence is, for the page to link.
const LicenseURL = "https://huggingface.co/LiquidAI/LFM3-1.7B-GGUF/blob/main/LICENSE"

// DownloadBytes is what turning Omni AI on downloads.
func DownloadBytes() int64 { return Llama.Size + Model.Size }
