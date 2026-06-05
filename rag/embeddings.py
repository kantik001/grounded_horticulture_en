"""Embeddings для multilingual-e5: обязательные префиксы query:/passage:."""

from langchain_huggingface import HuggingFaceEmbeddings

E5_MODEL = "intfloat/multilingual-e5-small"


class E5Embeddings(HuggingFaceEmbeddings):
    """intfloat/multilingual-e5-small ожидает префиксы при индексации и поиске."""

    def embed_documents(self, texts):
        return super().embed_documents([_passage(t) for t in texts])

    def embed_query(self, text):
        return super().embed_query(_query(text))


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


def get_embeddings() -> E5Embeddings:
    return E5Embeddings(model_name=E5_MODEL)
