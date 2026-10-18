package enginesetup

// Artifact is one file the installer downloads, pinned by size and SHA-256 so what is installed is what
// was measured (docs/eval/2026-10-18-clean-install.md): a changed or truncated file is refused.
type Artifact struct {
	Name   string
	URL    string
	Size   int64
	SHA256 string
}

// PythonVersion is the interpreter uv installs for the engine. It is pinned because uv would otherwise
// pick the newest one (3.15 at the time of writing), for which the libraries have no wheels.
const PythonVersion = "3.12"

// The pinned downloads. uv fetches its own Python (github.com) and the engine's libraries (pypi.org and
// files.pythonhosted.org) by itself; those are not pinned here but by the engine's uv.lock.
var (
	UV = Artifact{
		Name:   "uv 0.13.1",
		URL:    "https://github.com/astral-sh/uv/releases/download/0.13.1/uv-x86_64-pc-windows-msvc.zip",
		Size:   15308685,
		SHA256: "3b2661a463a5baea7f5076ea2b25ab71d77f36a6c0d809e5350651792f80a672",
	}
	Llama = Artifact{
		Name:   "llama.cpp b11719 (Vulkan)",
		URL:    "https://github.com/ggml-org/llama.cpp/releases/download/b11719/llama-b11719-bin-win-vulkan-x64.zip",
		Size:   32306255,
		SHA256: "08dde7f4133443ac629431c35f0435e267ca73251fc002cb95e10625424e2963",
	}
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

// DownloadBytes is roughly what turning file search on downloads (the pinned files plus about 80 MB of
// Python and libraries that uv fetches), for the window to say how big it is.
func DownloadBytes() int64 {
	const pythonAndLibraries = 80 << 20
	return UV.Size + Llama.Size + TextModel.Size + VisionModel.Size + pythonAndLibraries
}
