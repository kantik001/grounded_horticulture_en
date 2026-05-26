"""Загрузка config/crops.json — единый список культур для RAG и CV."""

import json
import os
from typing import Any, Dict, Optional

_PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

_CONFIG: Optional[Dict[str, Any]] = None


def _config_path() -> str:
    env = os.environ.get("CROPS_CONFIG_PATH")
    if env and os.path.isfile(env):
        return env
    for candidate in (
        os.path.join(_PROJECT_ROOT, "config", "crops.json"),
        "/config/crops.json",
    ):
        if os.path.isfile(candidate):
            return candidate
    return os.path.join(_PROJECT_ROOT, "config", "crops.json")


def load_crops_config() -> Dict[str, Any]:
    global _CONFIG
    if _CONFIG is not None:
        return _CONFIG
    path = _config_path()
    with open(path, encoding="utf-8") as f:
        _CONFIG = json.load(f)
    return _CONFIG


def default_crop_id() -> str:
    return load_crops_config().get("default_crop", "apple")


def normalize_crop_id(crop_id: Optional[str]) -> str:
    cid = (crop_id or "").strip().lower() or default_crop_id()
    crops = load_crops_config().get("crops", {})
    if cid not in crops:
        raise ValueError(f"Неизвестная культура: {crop_id}")
    return cid


def get_crop(crop_id: str) -> Dict[str, Any]:
    cid = normalize_crop_id(crop_id)
    return load_crops_config()["crops"][cid]


def list_crops() -> Dict[str, Any]:
    cfg = load_crops_config()
    return {
        "default_crop": cfg.get("default_crop", "apple"),
        "crops": cfg.get("crops", {}),
    }
