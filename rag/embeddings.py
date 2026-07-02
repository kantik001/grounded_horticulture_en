"""Embeddings for multilingual-e5: required query:/passage: prefixes."""

E5_MODEL = "intfloat/multilingual-e5-small"

_embeddings = None


def _passage(text: str) -> str:
    t = (text or "").strip()
    if t.lower().startswith("passage:"):
        return t
    return f"passage: {t}"


def _query(text: str) -> str:
    t = (text or "").strip()
    if t.lower().startswith("query:"):
        return t
    return f"query: {t}"


def reset_embeddings() -> None:
    """Clear cache (tests, model change)."""
    global _embeddings
    _embeddings = None


def get_embeddings():
    global _embeddings
    if _embeddings is not None:
        return _embeddings

    from langchain_huggingface import HuggingFaceEmbeddings

    class E5Embeddings(HuggingFaceEmbeddings):
        """intfloat/multilingual-e5-small expects prefixes at index and query time."""

        def embed_documents(self, texts):
            return super().embed_documents([_passage(t) for t in texts])

        def embed_query(self, text):
            return super().embed_query(_query(text))

    _embeddings = E5Embeddings(model_name=E5_MODEL)
    return _embeddings
