package enginesetup

// The downloads that are macOS programs, for Apple Silicon. Pinned like the rest: size and SHA-256, which are the digests GitHub
// publishes for the release files and were checked against the downloaded files. Both are .tar.gz holding one folder (Strip).
// llama.cpp's macOS build carries the Metal backend, so the model runs on the Mac's GPU.
var (
	UV = Artifact{
		Name:   "uv 0.13.1",
		URL:    "https://github.com/astral-sh/uv/releases/download/0.13.1/uv-aarch64-apple-darwin.tar.gz",
		Size:   14280170,
		SHA256: "cfe454573412ff7380d15fc7dae5997ab53ffce72fce0a78e91ddbc84001acd2",
		Strip:  true,
	}
	Llama = Artifact{
		Name:   "llama.cpp b11719 (Metal)",
		URL:    "https://github.com/ggml-org/llama.cpp/releases/download/b11719/llama-b11719-bin-macos-arm64.tar.gz",
		Size:   12292550,
		SHA256: "076a2b0d15253078b90bd520a82dc2edc6c17657968878876ba3def3e80f54ba",
		Strip:  true,
	}
)
