"""Request and response bodies. Unknown request fields are errors, not silently ignored."""

from __future__ import annotations

from pydantic import BaseModel


class HealthOut(BaseModel):
    ready: bool
    model: str
    device: str | None
    embedded_items: int
    version: str
    error: str | None
