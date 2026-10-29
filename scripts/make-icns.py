"""Packs the pictures inside build/windows/omni.ico into build/darwin/omni.icns, so the Mac icon is the Windows one.
Run it again only if the art changes:  python scripts/make-icns.py

An .icns file is a list of PNG pictures, each under a four-letter kind. The kinds below are the sizes the ico has; a retina size
(the "@2x" ones) is the picture of twice the size. Sizes of 512 and over are left out: the art stops at 256."""
import struct
from pathlib import Path

root = Path(__file__).resolve().parent.parent
ico = (root / "build/windows/omni.ico").read_bytes()
frames = {}
for i in range(struct.unpack("<H", ico[4:6])[0]):
    w, _, _, _, _, _, size, offset = struct.unpack("<BBBBHHII", ico[6 + 16 * i : 22 + 16 * i])
    frames[w or 256] = ico[offset : offset + size]

# kind -> pixels
kinds = {"icp4": 16, "icp5": 32, "icp6": 64, "ic07": 128, "ic08": 256, "ic11": 32, "ic12": 64, "ic13": 256}
body = b"".join(kind.encode() + struct.pack(">I", 8 + len(frames[px])) + frames[px] for kind, px in kinds.items())
out = root / "build/darwin/omni.icns"
out.write_bytes(b"icns" + struct.pack(">I", 8 + len(body)) + body)
print(f"wrote {out} ({out.stat().st_size} bytes)")
