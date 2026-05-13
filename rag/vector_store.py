# ----------------------------------------------------------------------
# Импорты
# ----------------------------------------------------------------------
import os
import glob
from langchain_community.document_loaders import TextLoader
from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_huggingface import HuggingFaceEmbeddings
from langchain_chroma import Chroma

# ----------------------------------------------------------------------
# Конфигурация (пути относительно корня репозитория, независимо от cwd)
# ----------------------------------------------------------------------
_PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
DATA_DIR = os.path.join(_PROJECT_ROOT, "data")
PERSIST_DIR = os.path.join(_PROJECT_ROOT, "chroma_db")

# Красивые названия статей (отображаются в источнике)
TITLES = {
    "article1.txt": "Эффективность выращивания яблони на террасированных склонах предгорной зоны КБР",
    "article2.txt": "ВЛИЯНИЕ БИО-ОРГАНО-МИНЕРАЛЬНОГО КОМПЛЕКСА АКМ НА БИОЛОГИЧЕСКУЮ АКТИВНОСТЬ ПОЧВЫ, ПРОДУКТИВНОСТЬ ЯБЛОНИ И КАЧЕСТВО ПЛОДОВ",
    "article3.txt": "ВЛИЯНИЕ УДОБРЕНИЯ НА МИНЕРАЛЬНОЕ ПИТАНИЕ, РОСТ, РАЗВИТИЕ И ПЛОДОНОШЕНИЕ ЯБЛОНИ КОЛОННОВИДНОЙ",
    # Добавьте другие файлы по аналогии
}

# ----------------------------------------------------------------------
# Вспомогательные функции
# ----------------------------------------------------------------------
def get_pretty_title(filename: str) -> str:
    """Возвращает красивое название статьи или имя файла, если нет в словаре."""
    return TITLES.get(filename, filename)

def load_all_documents():
    """Загружает все .txt файлы из DATA_DIR и добавляет в метаданные название статьи."""
    all_docs = []
    txt_files = glob.glob(os.path.join(DATA_DIR, "*.txt"))
    for file_path in txt_files:
        filename = os.path.basename(file_path)
        pretty_title = get_pretty_title(filename)
        print(f"Загружаю {filename} -> {pretty_title}")
        loader = TextLoader(file_path, encoding="utf-8")
        docs = loader.load()
        for doc in docs:
            if doc.metadata is None:
                doc.metadata = {}
            doc.metadata["filename"] = pretty_title
        all_docs.extend(docs)
    return all_docs

def create_vector_store():
    """Создаёт векторную базу из всех статей (разбивка на чанки, эмбеддинги)."""
    print("Создаю новую векторную базу...")
    documents = load_all_documents()
    text_splitter = RecursiveCharacterTextSplitter(chunk_size=500, chunk_overlap=50)
    docs = text_splitter.split_documents(documents)
    print(f"Фрагментов: {len(docs)}")
    embeddings = HuggingFaceEmbeddings(model_name="intfloat/multilingual-e5-small")
    vector_store = Chroma.from_documents(docs, embeddings, persist_directory=PERSIST_DIR)
    print(f"База сохранена в {PERSIST_DIR}")
    return vector_store

def load_vector_store():
    """Загружает существующую векторную базу или создаёт новую, если её нет."""
    embeddings = HuggingFaceEmbeddings(model_name="intfloat/multilingual-e5-small")
    if os.path.exists(PERSIST_DIR) and os.listdir(PERSIST_DIR):
        return Chroma(persist_directory=PERSIST_DIR, embedding_function=embeddings)
    else:
        return create_vector_store()

# ----------------------------------------------------------------------
# Основная функция поиска (без реранкинга, только косинусное сходство)
# ----------------------------------------------------------------------
def search(query: str, k: int = 8):
    """
    Ищет k наиболее релевантных фрагментов по смыслу.
    k = 8 даёт достаточно контекста для полного ответа.
    """
    vector_store = load_vector_store()
    return vector_store.similarity_search(query, k=k)