// Package llm is the language model behind Omni AI as a process: it installs the model and a llama.cpp server for it,
// starts that server on the graphics card (or the processor), and stops it again when it has been idle. It knows nothing
// about conversations; internal/chat asks it for the server's address and talks to it.
//
// It does not use the file-search engine's llama.cpp: the model needs a newer build than the one the embeddings were
// validated on (docs/decisions/0008-chat-runtime.md).
package llm
