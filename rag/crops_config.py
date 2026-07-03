"""Load config/crops.json — unified crop list for RAG and CV."""

import json
import os
from typing import Any, Dict, Optional

_PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

_CONFIG: Optional[Dict[str, Any]] = None
_CONFIG_MTIME: Optional[float] = None


def _config_path() -> str:
    """Resolve path to crops.json (env, local, or /config in Docker)."""
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
    """Read and cache crops.json; reload when the file changes on disk."""
    global _CONFIG, _CONFIG_MTIME
    path = _config_path()
    try:
        mtime = os.path.getmtime(path)
    except OSError:
        mtime = None
    if _CONFIG is not None and _CONFIG_MTIME == mtime:
        return _CONFIG
    with open(path, encoding="utf-8") as f:
        _CONFIG = json.load(f)
    _CONFIG_MTIME = mtime
    return _CONFIG


def reload_crops_config() -> Dict[str, Any]:
    """Clear crops.json cache (tests)."""
    global _CONFIG, _CONFIG_MTIME
    _CONFIG = None
    _CONFIG_MTIME = None
    return load_crops_config()


def default_crop_id() -> str:
    """Return default crop_id from config (usually apple)."""
    return load_crops_config().get("default_crop", "apple")


def crop_display_name(crop: Dict[str, Any], crop_id: str) -> str:
    """English display name when set, else Russian, else crop_id."""
    return crop.get("name_en") or crop.get("name_ru") or crop_id


def normalize_crop_id(crop_id: Optional[str]) -> str:
    """Lowercase crop_id and verify it exists in config."""
    cid = (crop_id or "").strip().lower() or default_crop_id()
    crops = load_crops_config().get("crops", {})
    if cid not in crops:
        raise ValueError(f"Unknown crop: {crop_id}")
    return cid


def get_crop(crop_id: str) -> Dict[str, Any]:
    """Return one crop entry from crops.json."""
    cid = normalize_crop_id(crop_id)
    return load_crops_config()["crops"][cid]


def list_crops() -> Dict[str, Any]:
    """Return all crops for GET /crops-style APIs."""
    cfg = load_crops_config()
    return {
        "default_crop": cfg.get("default_crop", "apple"),
        "crops": cfg.get("crops", {}),
    }
