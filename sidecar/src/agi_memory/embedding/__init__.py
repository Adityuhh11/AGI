"""Embedding models: the `Embedder` contract and its implementations.

`LlamaEmbedder` (`agi_memory.embedding.llama`) and `LlamaServer`
(`agi_memory.embedding.llama_server`) are not re-exported here: tests and tools that only need
the contract should not pull in the HTTP client and the process code.
"""

from agi_memory.embedding.base import Embedder
from agi_memory.embedding.fake import FakeEmbedder

__all__ = ["Embedder", "FakeEmbedder"]
