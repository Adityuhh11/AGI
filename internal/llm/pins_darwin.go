package llm

import "omni/internal/enginesetup"

// Llama is the llama.cpp server of the macOS build for Apple Silicon (Metal). Its size and SHA-256 are the digests GitHub
// publishes for the release file (b11970 is the build the chat model needs; docs/eval/2026-10-28-chat-model.md).
var Llama = enginesetup.Artifact{
	Name:   "llama.cpp b11970 (Metal)",
	URL:    "https://github.com/ggml-org/llama.cpp/releases/download/b11970/llama-b11970-bin-macos-arm64.tar.gz",
	Size:   16465031,
	SHA256: "7663cc0b4c23e5659a9cf129f8af085bc042faccc36ef39aee8e57c7ef49d0b1",
	Strip:  true,
}
