# ----------------------------------------------------------------------
# Векторное хранилище Chroma: статьи по культурам data/{crop_id}/*.txt
# ----------------------------------------------------------------------
import glob
import json
import os
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
from langchain_huggingface import HuggingFaceEmbeddings
from langchain_text_splitters import RecursiveCharacterTextSplitter

from rag.crops_config import list_crops, normalize_crop_id

_PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
DATA_DIR = os.path.join(_PROJECT_ROOT, "data")
PERSIST_DIR = os.path.join(_PROJECT_ROOT, "chroma_db")

_vector_store = None
_titles_cache = None


# Сбрасывает in-memory кэш Chroma перед принудительной переиндексацией.
def reset_vector_store():
    global _vector_store
    _vector_store = None


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


# Человекочитаемое название статьи по crop_id и имени файла.
def get_pretty_title(crop_id: str, filename: str) -> str:
    return _titles_map().get(crop_id, {}).get(filename, filename)


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
    pretty_title = get_pretty_title(crop_id, filename)
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


# Полная переиндексация: split → embeddings → сохранение в chroma_db/.
def create_vector_store():
    print("Создаю новую векторную базу (мультикультура)...")
    documents = load_all_documents()
    if not documents:
        print("Нет статей для индексации.")
        return None
    text_splitter = RecursiveCharacterTextSplitter(chunk_size=500, chunk_overlap=50)
    docs = text_splitter.split_documents(documents)
    print(f"Фрагментов: {len(docs)}")
    embeddings = HuggingFaceEmbeddings(model_name="intfloat/multilingual-e5-small")
    store = Chroma.from_documents(docs, embeddings, persist_directory=PERSIST_DIR)
    print(f"База сохранена в {PERSIST_DIR}")
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
    embeddings = HuggingFaceEmbeddings(model_name="intfloat/multilingual-e5-small")

    if force and os.path.isdir(PERSIST_DIR):
        import shutil

        print("FORCE_RAG_REINDEX: удаляю старую chroma_db")
        shutil.rmtree(PERSIST_DIR, ignore_errors=True)

    if os.path.exists(PERSIST_DIR) and os.listdir(PERSIST_DIR):
        _vector_store = Chroma(persist_directory=PERSIST_DIR, embedding_function=embeddings)
    else:
        _vector_store = create_vector_store()
    return _vector_store


# Семантический поиск top-k фрагментов только по выбранной культуре.
def search(query: str, crop_id: str, k: int = 8):
    crop_id = normalize_crop_id(crop_id)
    store = load_vector_store()
    if store is None:
        return []
    return store.similarity_search(
        query,
        k=k,
        filter={"crop_id": crop_id},
    )
