# ----------------------------------------------------------------------
# Chroma vector store: articles per crop in data/{crop_id}/*.txt
# ----------------------------------------------------------------------
import glob
import os
import sys
import threading


def _ensure_utf8_stdout() -> None:
    """Force UTF-8 stdout (Windows consoles may default to a legacy codepage)."""
    if hasattr(sys.stdout, "reconfigure"):
        try:
            sys.stdout.reconfigure(encoding="utf-8")
        except Exception:
            pass


_ensure_utf8_stdout()

from langchain_chroma import Chroma
from langchain_community.document_loaders import TextLoader
from langchain_core.documents import Document

from rag.bm25_store import (
    bm25_search,
    build_bm25_indexes,
    get_chunk_document,
    load_bm25_indexes,
    remove_bm25_index,
    reset_bm25_store,
    save_bm25_indexes,
)
from rag.chunking import assign_chunk_ids, chunk_id_for, diversify_fragments, split_documents
from rag.crops_config import list_crops, normalize_crop_id
from rag.embeddings import get_embeddings
from rag.hybrid import hybrid_enabled, rerank_for_category, rrf_merge
from rag.query_expand import expand_query
from rag.reranker import RERANK_TOP_N, rerank_documents
from rag.titles import get_pretty_title

_PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
DATA_DIR = os.path.join(_PROJECT_ROOT, "data")
PERSIST_DIR = os.path.join(_PROJECT_ROOT, "chroma_db")

_vector_store = None
_vector_store_lock = threading.Lock()

# How many candidates to take from Chroma/BM25 before rerank and per-article dedup.
FETCH_K = int(os.environ.get("RAG_FETCH_K", "16"))
BM25_FETCH_K = int(os.environ.get("RAG_BM25_FETCH_K", str(FETCH_K)))


# Clears in-memory Chroma and BM25 cache before forced reindexing.
def reset_vector_store():
    global _vector_store
    _vector_store = None
    reset_bm25_store()


# data/ folder meta files that are not knowledge base content and are not indexed.
_SKIP_FILENAMES = {"readme.txt"}


def _is_indexable(file_path: str) -> bool:
    """False for meta files (README) that must not be indexed."""
    return os.path.basename(file_path).lower() not in _SKIP_FILENAMES


# Collects .txt from data/{crop}/ and legacy data/*.txt at repo root (like apple).
def load_all_documents():
    all_docs = []
    crops = list_crops().get("crops", {}).keys()

    for crop_id in crops:
        crop_dir = os.path.join(DATA_DIR, crop_id)
        if not os.path.isdir(crop_dir):
            continue
        for file_path in glob.glob(os.path.join(crop_dir, "*.txt")):
            if not _is_indexable(file_path):
                continue
            all_docs.extend(_load_file(crop_id, file_path))

    for file_path in glob.glob(os.path.join(DATA_DIR, "*.txt")):
        if not _is_indexable(file_path):
            continue
        all_docs.extend(_load_file("apple", file_path))

    return all_docs


# Reads one .txt and sets metadata: filename, crop_id, source_file.
def _load_file(crop_id: str, file_path: str):
    filename = os.path.basename(file_path)
    pretty_title = get_pretty_title(crop_id, filename, file_path=file_path)
    print(f"Loading [{crop_id}] {filename} -> {pretty_title}")
    loader = TextLoader(file_path, encoding="utf-8")
    docs = loader.load()
    for doc in docs:
        if doc.metadata is None:
            doc.metadata = {}
        doc.metadata["filename"] = pretty_title
        doc.metadata["crop_id"] = crop_id
        doc.metadata["source_file"] = filename
    return docs


# Full reindex: split -> embeddings + BM25 -> save to chroma_db/ and bm25_db/.
def create_vector_store():
    print("Building new vector store (multi-crop)...")
    documents = load_all_documents()
    if not documents:
        print("No articles to index.")
        return None
    chunks = assign_chunk_ids(split_documents(documents))
    print(f"Fragments: {len(chunks)}")
    embeddings = get_embeddings()
    store = Chroma.from_documents(chunks, embeddings, persist_directory=PERSIST_DIR)
    print(f"Store saved to {PERSIST_DIR}")
    save_bm25_indexes(build_bm25_indexes(chunks))
    return store


# Opens an existing Chroma store or creates a new one (respects FORCE_RAG_REINDEX).
# The lock serializes cold-start loading and /admin/reindex against concurrent searches.
def load_vector_store(force_reindex: bool = False):
    global _vector_store
    if _vector_store is not None and not force_reindex:
        return _vector_store

    with _vector_store_lock:
        if _vector_store is not None and not force_reindex:
            return _vector_store

        force = force_reindex or os.environ.get("FORCE_RAG_REINDEX", "").lower() in (
            "1",
            "true",
            "yes",
        )
        embeddings = get_embeddings()

        if force and os.path.isdir(PERSIST_DIR):
            import shutil

            print("FORCE_RAG_REINDEX: removing old chroma_db")
            shutil.rmtree(PERSIST_DIR, ignore_errors=True)
            remove_bm25_index()

        if os.path.exists(PERSIST_DIR) and os.listdir(PERSIST_DIR):
            _vector_store = Chroma(persist_directory=PERSIST_DIR, embedding_function=embeddings)
            load_bm25_indexes()
        else:
            _vector_store = create_vector_store()
        return _vector_store


def _chunk_key(doc: Document) -> str:
    """chunk_id from metadata, computed on the fly if absent."""
    return doc.metadata.get("chunk_id") or chunk_id_for(doc)


def _collect_candidates(
    merged_ids: list[str],
    doc_map: dict[str, Document],
    crop_id: str,
    limit: int,
) -> list[Document]:
    """Resolve merged chunk_ids to Documents (dedup, up to limit)."""
    candidates: list[Document] = []
    seen: set[str] = set()
    for cid in merged_ids:
        if cid in seen:
            continue
        seen.add(cid)
        doc = doc_map.get(cid) or get_chunk_document(cid, crop_id)
        if doc is None:
            continue
        candidates.append(doc)
        if len(candidates) >= limit:
            break
    return candidates


# Hybrid search: vector + BM25 (RRF) -> reranker (by category) -> diversify top-k.
def search(query: str, crop_id: str, k: int = 8, category: str | None = None):
    crop_id = normalize_crop_id(crop_id)
    store = load_vector_store()
    if store is None:
        return []

    search_query = expand_query(query)
    fetch_k = max(k * 3, FETCH_K)
    vector_docs = store.similarity_search(
        search_query,
        k=fetch_k,
        filter={"crop_id": crop_id},
    )
    if not vector_docs:
        return []

    doc_map = {_chunk_key(d): d for d in vector_docs}
    vector_ids = list(doc_map.keys())

    use_hybrid = hybrid_enabled() and load_bm25_indexes() is not None
    if use_hybrid:
        bm25_ids = bm25_search(search_query, crop_id, max(fetch_k, BM25_FETCH_K))
        merged_ids = rrf_merge([vector_ids, bm25_ids])
    else:
        merged_ids = vector_ids

    use_rerank = rerank_for_category(category)
    candidate_limit = max(fetch_k, RERANK_TOP_N) if use_rerank else fetch_k
    candidates = _collect_candidates(merged_ids, doc_map, crop_id, candidate_limit)

    if use_rerank and len(candidates) > 1:
        candidates = rerank_documents(search_query, candidates)

    return diversify_fragments(candidates, limit=k)
