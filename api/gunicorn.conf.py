"""Gunicorn: production WSGI для classifier (1 worker + threads — общая память ML-моделей)."""

import os

bind = f"0.0.0.0:{os.environ.get('CLASSIFIER_PORT', '5000')}"
workers = max(1, int(os.environ.get("GUNICORN_WORKERS", "1")))
threads = max(1, int(os.environ.get("GUNICORN_THREADS", "4")))
timeout = max(30, int(os.environ.get("GUNICORN_TIMEOUT", "300")))
graceful_timeout = 30
keepalive = 5
worker_class = "gthread" if threads > 1 else "sync"
preload_app = workers == 1
accesslog = "-"
errorlog = "-"
capture_output = True


def when_ready(server):
    """Прогрев RAG после старта master (до приёма трафика)."""
    from rag.warmup import warmup_rag

    warmup_rag()
