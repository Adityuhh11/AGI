package enginesetup

// Artifact is one file the installer downloads, pinned by size and SHA-256 so what is installed is what
// was measured (docs/eval/2026-10-18-clean-install.md): a changed or truncated file is refused.
type Artifact struct {
	Name   string
	URL    string
	Size   int64
	SHA256 string
	// Strip says the archive wraps everything in one folder that is dropped when it is unpacked (the macOS tarballs do).
	Strip bool
}

// PythonVersion is the interpreter uv installs for the engine. It is pinned because uv would otherwise
// pick the newest one (3.15 at the time of writing), for which the libraries have no wheels.
const PythonVersion = "3.12"

// The pinned downloads that are the same on every system. uv (UV) and llama.cpp (Llama) are pinned per system in pins_windows.go and
// pins_darwin.go. uv fetches its own Python (github.com) and the engine's libraries (pypi.org and
// files.pythonhosted.org) by itself; those are not pinned here but by the engine's uv.lock.
var (
	TextModel = Artifact{
		Name:   "EmbeddingGemma 2, text (Q8_0)",
		URL:    "https://huggingface.co/ggml-org/embeddinggemma-2-GGUF/resolve/bfcd298762cc34d0357ece5ebdd31791a3a374d8/embeddinggemma-2-Q8_0.gguf",
		Size:   309855456,
		SHA256: "2188ac1deca4b77dffefd603c2776a9d76d9d74ec01841392982ebb840b09135",
	}
	VisionModel = Artifact{
		Name:   "EmbeddingGemma 2, images (Q8_0)",
		URL:    "https://huggingface.co/ggml-org/embeddinggemma-2-GGUF/resolve/bfcd298762cc34d0357ece5ebdd31791a3a374d8/mmproj-embeddinggemma-2-Q8_0.gguf",
		Size:   554821024,
		SHA256: "c4a8a52691ecef40618438928bdf9e68379b854e24166f292592353db0aab64f",
	}
)

// Hosts are the servers turning file search on talks to, for the window to show before the person agrees.
var Hosts = []string{"github.com", "pypi.org", "files.pythonhosted.org", "huggingface.co"}

// DownloadBytes is roughly what turning file search on downloads (the pinned files plus about 110 MB of
// Python and libraries that uv fetches, 27 MB of it PyAV for videos), for the window to say how big it is.
func DownloadBytes() int64 {
	const pythonAndLibraries = 110 << 20
	return UV.Size + Llama.Size + TextModel.Size + VisionModel.Size + pythonAndLibraries
}
