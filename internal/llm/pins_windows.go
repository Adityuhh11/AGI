package llm

import "omni/internal/enginesetup"

// Llama is the llama.cpp server of the Windows build (Vulkan, so any graphics card).
var (
	Llama = enginesetup.Artifact{
		Name:   "llama.cpp b11970 (Vulkan)",
		URL:    "https://github.com/ggml-org/llama.cpp/releases/download/b11970/llama-b11970-bin-win-vulkan-x64.zip",
		Size:   37346727,
		SHA256: "2d043e717386c03caa1f150fbcf6c37b513d9872ed44f5d41706b2fecc14617b",
	}
)
