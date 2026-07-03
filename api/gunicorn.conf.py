"""Gunicorn: production WSGI for classifier (1 worker + threads — shared ML model memory)."""

import os

bind = f"0.0.0.0:{os.environ.get('CLASSIFIER_PORT', '5000')}"
workers = max(1, int(os.environ.get("GUNICORN_WORKERS", "1")))
threads = max(1, int(os.environ.get("GUNICORN_THREADS", "4")))
timeout = max(30, int(os.environ.get("GUNICORN_TIMEOUT", "300")))
graceful_timeout = 30
keepalive = 5
worker_class = "gthread" if threads > 1 else "sync"
# PyTorch / sentence-transformers must not initialize in master before fork — worker deadlock.
preload_app = False
accesslog = "-"
errorlog = "-"
capture_output = True


def post_fork(server, worker):
    """Warm up RAG in the worker process that serves /rag/context."""
    from rag.warmup import warmup_rag

    warmup_rag()
