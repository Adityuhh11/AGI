package enginesetup

// The downloads that are Windows programs. (Pinned like the rest: size and SHA-256.)
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
)
