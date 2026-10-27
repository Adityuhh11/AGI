"""A growable, memory-mapped matrix of float32 vectors stored in one file.

Layout: a 64-byte header (magic, version, dim, capacity) followed by `capacity` rows of
`dim` little-endian float32. Row r starts at byte 64 + r * dim * 4. The file grows by
doubling. Windows cannot resize a mapped file, so growth unmaps, truncates and remaps; that
is only safe because no numpy view of the mapping ever leaves this class (`scores` and
`read` return fresh arrays).

Not thread-safe: the owner (`FlatStore`) serialises access.
"""

from __future__ import annotations

import mmap
import os
import struct
from collections.abc import Sequence
from pathlib import Path

import numpy as np

_MAGIC = b"OMNIVEC1"
_VERSION = 1
_HEADER = struct.Struct("<8sIIQ")  # magic, version, dim, capacity
HEADER_SIZE = 64
_ITEM_SIZE = 4  # bytes per float32


class VectorFileError(Exception):
    """The file is not a valid vector file for the requested dimension."""


class VectorFile:
    """Rows of `dim` float32 values addressed by row number."""

    def __init__(self, path: Path, dim: int, *, initial_capacity: int = 1024) -> None:
        if initial_capacity < 1:
            raise ValueError(f"initial_capacity must be >= 1, got {initial_capacity}")
        self._path = path
        self._dim = dim
        existed = path.exists()
        # Unbuffered on purpose: on Windows a buffered handle's read-ahead makes close() fail with
        # EINVAL once the file has been mapped and fsynced. We only touch the header and truncate.
        # Closed in close() or on a failed open below.
        self._file = open(path, "r+b" if existed else "w+b", buffering=0)  # noqa: SIM115
        try:
            if existed:
                self._capacity = self._read_header()
            else:
                self._capacity = initial_capacity
                self._file.truncate(self._size_for(self._capacity))
                self._write_header()
            self._map()
        except BaseException:
            self._file.close()
            raise

    @property
    def dim(self) -> int:
        return self._dim

    @property
    def capacity(self) -> int:
        return self._capacity

    def _size_for(self, capacity: int) -> int:
        return HEADER_SIZE + capacity * self._dim * _ITEM_SIZE

    def _write_header(self) -> None:
        header = _HEADER.pack(_MAGIC, _VERSION, self._dim, self._capacity)
        self._file.seek(0)
        self._file.write(header.ljust(HEADER_SIZE, b"\0"))
        self._file.flush()

    def _read_header(self) -> int:
        self._file.seek(0)
        raw = self._file.read(_HEADER.size)
        if len(raw) < _HEADER.size:
            raise VectorFileError(f"{self._path}: truncated header")
        magic, version, dim, capacity = _HEADER.unpack(raw)
        if magic != _MAGIC or version != _VERSION:
            raise VectorFileError(f"{self._path}: not a vector file (magic/version mismatch)")
        if dim != self._dim:
            raise VectorFileError(f"{self._path}: holds dim {dim}, expected dim {self._dim}")
        if capacity < 1:  # would make ensure_capacity's doubling loop forever
            raise VectorFileError(f"{self._path}: header claims capacity {capacity}")
        if os.fstat(self._file.fileno()).st_size < self._size_for(capacity):
            raise VectorFileError(f"{self._path}: file is shorter than its header claims")
        return capacity

    def _map(self) -> None:
        self._mm = mmap.mmap(self._file.fileno(), self._size_for(self._capacity))
        flat = np.frombuffer(
            self._mm, dtype="<f4", offset=HEADER_SIZE, count=self._capacity * self._dim
        )
        self._matrix: np.ndarray | None = flat.reshape(self._capacity, self._dim)

    def _unmap(self) -> None:
        self._matrix = None  # drop every numpy view first, or mmap.close() raises BufferError
        self._mm.flush()
        self._mm.close()

    @property
    def _rows(self) -> np.ndarray:
        if self._matrix is None:
            raise ValueError("vector file is closed")
        return self._matrix

    def ensure_capacity(self, rows: int) -> None:
        """Grow (doubling) until at least `rows` rows fit."""
        if rows <= self._capacity:
            return
        capacity = self._capacity
        while capacity < rows:
            capacity *= 2
        previous = self._capacity
        self._unmap()
        try:
            self._file.truncate(self._size_for(capacity))
            self._capacity = capacity
            self._write_header()
        except BaseException:
            self._capacity = previous  # e.g. disk full: keep serving the old, still valid size
            raise
        finally:
            self._map()  # always leave the file mapped, or the store is dead after one failure

    def write_rows(self, rows: Sequence[int], vectors: np.ndarray) -> None:
        """Store `vectors[i]` at row `rows[i]`, growing the file if needed."""
        expected = (len(rows), self._dim)
        if vectors.shape != expected:
            raise ValueError(f"vectors must have shape {expected}, got {vectors.shape}")
        if len(rows) == 0:
            return
        index = np.asarray(rows, dtype=np.intp)
        self.ensure_capacity(int(index.max()) + 1)
        self._rows[index] = vectors

    def read(self, row: int) -> np.ndarray:
        """A copy of one row."""
        return self._rows[row].copy()

    def scores(self, query: np.ndarray, upto: int) -> np.ndarray:
        """Dot products of `query` with rows `[0, upto)`: a fresh array, safe to keep."""
        return self._rows[:upto] @ query

    def flush(self) -> None:
        """Push written rows to disk."""
        self._mm.flush()
        os.fsync(self._file.fileno())

    def close(self) -> None:
        if self._file.closed:
            return
        self._unmap()
        self._file.close()
