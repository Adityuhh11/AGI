"""Embedding models: the `Embedder` contract and its implementations.

`LlamaEmbedder` (`omni_memory.embedding.llama`) and `LlamaServer`
(`omni_memory.embedding.llama_server`) are not re-exported here: tests and tools that only need
the contract should not pull in the HTTP client and the process code.
"""

from omni_memory.embedding.base import Embedder
from omni_memory.embedding.fake import FakeEmbedder

__all__ = ["Embedder", "FakeEmbedder"]
