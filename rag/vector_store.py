# ----------------------------------------------------------------------
# Векторное хранилище Chroma: статьи по культурам data/{crop_id}/*.txt
# ----------------------------------------------------------------------
import glob
import json
import os
import re
import sys


def _ensure_utf8_stdout() -> None:
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
from rag.chunking import assign_chunk_ids, chunk_id_for, split_documents
from rag.crops_config import list_crops, normalize_crop_id
from rag.embeddings import get_embeddings
from rag.hybrid import hybrid_enabled, rerank_enabled, rrf_merge
from rag.reranker import RERANK_TOP_N, rerank_documents

_PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
DATA_DIR = os.path.join(_PROJECT_ROOT, "data")
PERSIST_DIR = os.path.join(_PROJECT_ROOT, "chroma_db")

_vector_store = None
_titles_cache = None

# Сколько кандидатов взять из Chroma/BM25 перед rerank и дедупликацией по статьям.
FETCH_K = int(os.environ.get("RAG_FETCH_K", "24"))
BM25_FETCH_K = int(os.environ.get("RAG_BM25_FETCH_K", str(FETCH_K)))
MAX_CHUNKS_PER_SOURCE = int(os.environ.get("RAG_MAX_CHUNKS_PER_SOURCE", "2"))


# Сбрасывает in-memory кэш Chroma и BM25 перед принудительной переиндексацией.
def reset_vector_store():
    global _vector_store
    _vector_store = None
    reset_bm25_store()


# Загружает config/article_titles.json для красивых имён статей.
def _titles_map() -> dict:
    global _titles_cache
    if _titles_cache is not None:
        return _titles_cache
    path = os.path.join(_PROJECT_ROOT, "config", "article_titles.json")
    if not os.path.isfile(path):
        _titles_cache = {}
        return _titles_cache
    with open(path, encoding="utf-8") as f:
        _titles_cache = json.load(f)
    return _titles_cache


# Имя файла → читаемый заголовок, если нет записи в article_titles.json.
def _title_from_slug(filename: str) -> str:
    stem = filename.replace(".txt", "")
    m = re.match(r"article\d+_(.+)", stem, re.I)
    if not m:
        return filename
    words = m.group(1).replace("_", " ").strip()
    if not words:
        return filename
    return words[0].upper() + words[1:]


# Читает «- Заголовок:» из шапки статьи (первые 20 строк).
def _title_from_file_metadata(file_path: str) -> str | None:
    try:
        with open(file_path, encoding="utf-8") as f:
            for _ in range(20):
                line = f.readline()
                if not line.startswith("- Заголовок:"):
                    continue
                title = line.split(":", 1)[1].strip()
                if len(title) >= 12 and not title.endswith("…"):
                    return title[:160]
    except OSError:
        pass
    return None


# Человекочитаемое название статьи по crop_id и имени файла.
def get_pretty_title(crop_id: str, filename: str, file_path: str | None = None) -> str:
    mapped = _titles_map().get(crop_id, {}).get(filename)
    if mapped:
        return mapped
    if file_path:
        from_meta = _title_from_file_metadata(file_path)
        if from_meta:
            return from_meta
    if filename.endswith(".txt") and filename.startswith("article"):
        return _title_from_slug(filename)
    return filename


# Мета-файлы папок data/, которые не являются базой знаний и не индексируются.
_SKIP_FILENAMES = {"readme.txt"}


def _is_indexable(file_path: str) -> bool:
    return os.path.basename(file_path).lower() not in _SKIP_FILENAMES


# Собирает .txt из data/{crop}/ и устаревшие data/*.txt в корне (как apple).
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


# Читает один .txt и проставляет metadata: filename, crop_id, source_file.
def _load_file(crop_id: str, file_path: str):
    filename = os.path.basename(file_path)
    pretty_title = get_pretty_title(crop_id, filename, file_path=file_path)
    print(f"Загружаю [{crop_id}] {filename} -> {pretty_title}")
    loader = TextLoader(file_path, encoding="utf-8")
    docs = loader.load()
    for doc in docs:
        if doc.metadata is None:
            doc.metadata = {}
        doc.metadata["filename"] = pretty_title
        doc.metadata["crop_id"] = crop_id
        doc.metadata["source_file"] = filename
    return docs


# Полная переиндексация: split → embeddings + BM25 → сохранение в chroma_db/ и bm25_db/.
def create_vector_store():
    print("Создаю новую векторную базу (мультикультура)...")
    documents = load_all_documents()
    if not documents:
        print("Нет статей для индексации.")
        return None
    chunks = assign_chunk_ids(split_documents(documents))
    print(f"Фрагментов: {len(chunks)}")
    embeddings = get_embeddings()
    store = Chroma.from_documents(chunks, embeddings, persist_directory=PERSIST_DIR)
    print(f"База сохранена в {PERSIST_DIR}")
    save_bm25_indexes(build_bm25_indexes(chunks))
    return store


# Открывает существующую Chroma или создаёт новую (с учётом FORCE_RAG_REINDEX).
def load_vector_store(force_reindex: bool = False):
    global _vector_store
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

        print("FORCE_RAG_REINDEX: удаляю старую chroma_db")
        shutil.rmtree(PERSIST_DIR, ignore_errors=True)
        remove_bm25_index()

    if os.path.exists(PERSIST_DIR) and os.listdir(PERSIST_DIR):
        _vector_store = Chroma(persist_directory=PERSIST_DIR, embedding_function=embeddings)
        load_bm25_indexes()
    else:
        _vector_store = create_vector_store()
    return _vector_store


# Не более max_per_source чанков с одной статьи; сохраняет порядок релевантности.
def diversify_fragments(docs, limit: int, max_per_source: int = MAX_CHUNKS_PER_SOURCE):
    if max_per_source <= 0:
        return docs[:limit]
    picked = []
    counts: dict[str, int] = {}
    for doc in docs:
        src = doc.metadata.get("source_file") or doc.metadata.get("filename") or ""
        if counts.get(src, 0) >= max_per_source:
            continue
        picked.append(doc)
        counts[src] = counts.get(src, 0) + 1
        if len(picked) >= limit:
            break
    return picked


def _chunk_key(doc: Document) -> str:
    return doc.metadata.get("chunk_id") or chunk_id_for(doc)


def _collect_candidates(
    merged_ids: list[str],
    doc_map: dict[str, Document],
    crop_id: str,
    limit: int,
) -> list[Document]:
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


# Гибридный поиск: vector + BM25 (RRF) → reranker → diversify top-k.
def search(query: str, crop_id: str, k: int = 8):
    crop_id = normalize_crop_id(crop_id)
    store = load_vector_store()
    if store is None:
        return []

    fetch_k = max(k * 3, FETCH_K)
    vector_docs = store.similarity_search(
        query,
        k=fetch_k,
        filter={"crop_id": crop_id},
    )
    if not vector_docs:
        return []

    doc_map = {_chunk_key(d): d for d in vector_docs}
    vector_ids = list(doc_map.keys())

    use_hybrid = hybrid_enabled() and load_bm25_indexes() is not None
    if use_hybrid:
        bm25_ids = bm25_search(query, crop_id, max(fetch_k, BM25_FETCH_K))
        merged_ids = rrf_merge([vector_ids, bm25_ids])
    else:
        merged_ids = vector_ids

    rerank_pool = max(fetch_k, RERANK_TOP_N)
    candidates = _collect_candidates(merged_ids, doc_map, crop_id, rerank_pool)

    if rerank_enabled() and len(candidates) > 1:
        candidates = rerank_documents(query, candidates)

    return diversify_fragments(candidates, limit=k)
