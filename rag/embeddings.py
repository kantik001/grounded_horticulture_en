"""Embeddings for multilingual-e5: required query:/passage: prefixes."""

import threading

E5_MODEL = "intfloat/multilingual-e5-small"

_embeddings = None
_embeddings_lock = threading.Lock()


def _passage(text: str) -> str:
    """Prepend the e5 'passage:' prefix if missing."""
    t = (text or "").strip()
    if t.lower().startswith("passage:"):
        return t
    return f"passage: {t}"


def _query(text: str) -> str:
    """Prepend the e5 'query:' prefix if missing."""
    t = (text or "").strip()
    if t.lower().startswith("query:"):
        return t
    return f"query: {t}"


def reset_embeddings() -> None:
    """Clear cache (tests, model change)."""
    global _embeddings
    _embeddings = None


def get_embeddings():
    """Lazily load and cache the e5 embeddings model (thread-safe singleton)."""
    global _embeddings
    if _embeddings is not None:
        return _embeddings

    # Double-checked locking: model load is slow and must happen once even
    # when several gunicorn threads hit a cold worker simultaneously.
    with _embeddings_lock:
        if _embeddings is not None:
            return _embeddings

        from langchain_huggingface import HuggingFaceEmbeddings

        class E5Embeddings(HuggingFaceEmbeddings):
            """intfloat/multilingual-e5-small expects prefixes at index and query time."""

            def embed_documents(self, texts):
                """Embed texts with the 'passage:' prefix applied."""
                return super().embed_documents([_passage(t) for t in texts])

            def embed_query(self, text):
                """Embed a query with the 'query:' prefix applied."""
                return super().embed_query(_query(text))

        _embeddings = E5Embeddings(model_name=E5_MODEL)
        return _embeddings
