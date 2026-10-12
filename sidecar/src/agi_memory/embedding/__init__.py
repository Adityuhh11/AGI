"""Embedding models: the `Embedder` contract and its implementations.

`GemmaEmbedder` is deliberately not re-exported here: importing it pulls in torch, which
tests and tools that only need the contract should not pay for. Import it from
`agi_memory.embedding.gemma`.
"""

from agi_memory.embedding.base import Embedder
from agi_memory.embedding.fake import FakeEmbedder

__all__ = ["Embedder", "FakeEmbedder"]
