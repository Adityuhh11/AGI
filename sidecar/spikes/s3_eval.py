"""S3: the golden-set eval with llama-server as the embedder.

    python -m spikes.s3_eval <label> <server-url> [image-format]
"""

import sys

from agi_memory.eval.runner import run
from spikes.llama_embedder import LlamaEmbedder

label, url = sys.argv[1], sys.argv[2]
image_format = sys.argv[3] if len(sys.argv) > 3 else "PNG"
sys.exit(run(["--label", label], lambda _cpu: LlamaEmbedder(url, image_format=image_format), "2026-10-18"))
