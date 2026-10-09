# AGI

A local-first, AI-native launcher for Windows: a smarter, private Raycast.

Press a hotkey, ask anything, act on whatever is on your screen. Everything runs on
your machine; cloud LLMs are optional and configured by you.

## Status

Planning. See [docs/](docs/) for design specs and decisions.

## Architecture (planned)

```
[Wails v3 UI: Svelte]  <->  [Go core: hotkeys, context capture, routing]
                               |-- LLM: OpenAI-compatible client -> Ollama | API provider
                               '-- Python sidecar (localhost): EmbeddingGemma 2 (+ decision model later)
```

| Layer    | Tech                          | Latency | Job                          |
|----------|-------------------------------|---------|------------------------------|
| Reflex   | EmbeddingGemma 2 (local)      | ~10 ms  | What is this? What is it like? |
| Instinct | Decision model (local or API) | ~40 ms  | Which action? Does it matter?  |
| Thought  | LLM (Ollama or API)           | 1-5 s   | Write, explain, answer         |
