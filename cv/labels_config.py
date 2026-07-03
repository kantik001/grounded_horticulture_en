"""CV class labels per crop from config/cv_class_labels.json."""

import json
import os
from typing import Dict, List, Optional

_PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

_LABELS_CACHE: Optional[Dict[str, List[str]]] = None
_LABELS_MTIME: Optional[float] = None

# Fallback when the labels file is unavailable (backward compatibility).
_FALLBACK_APPLE = [
    "healthy_apple",
    "apple_scab",
    "black_rot",
    "cedar_apple_rust",
    "healthy_leaf",
    "powdery_mildew",
    "fire_blight",
    "bitter_rot",
    "blue_mold",
    "brown_rot",
]


def _labels_path() -> str:
    """Path to cv_class_labels.json (env override, then standard locations)."""
    env = os.environ.get("CV_CLASS_LABELS_PATH")
    if env and os.path.isfile(env):
        return env
    for candidate in (
        os.path.join(_PROJECT_ROOT, "config", "cv_class_labels.json"),
        "/config/cv_class_labels.json",
    ):
        if os.path.isfile(candidate):
            return candidate
    return os.path.join(_PROJECT_ROOT, "config", "cv_class_labels.json")


def _load_labels_file() -> Dict[str, List[str]]:
    """Load labels file with mtime-based caching; falls back to apple defaults."""
    global _LABELS_CACHE, _LABELS_MTIME
    path = _labels_path()
    try:
        mtime = os.path.getmtime(path)
    except OSError:
        return {"apple": list(_FALLBACK_APPLE)}
    if _LABELS_CACHE is not None and _LABELS_MTIME == mtime:
        return _LABELS_CACHE
    with open(path, encoding="utf-8") as f:
        raw = json.load(f)
    out: Dict[str, List[str]] = {}
    for crop_id, labels in raw.items():
        if isinstance(labels, list) and labels:
            out[str(crop_id).strip().lower()] = [str(x) for x in labels]
    if "apple" not in out:
        out["apple"] = list(_FALLBACK_APPLE)
    _LABELS_CACHE = out
    _LABELS_MTIME = mtime
    return out


def default_class_labels_for_crop(crop_id: str) -> List[str]:
    """Class label list for a crop (order = training index)."""
    cid = (crop_id or "apple").strip().lower() or "apple"
    catalog = _load_labels_file()
    if cid in catalog:
        return list(catalog[cid])
    if "apple" in catalog:
        return list(catalog["apple"])
    return list(_FALLBACK_APPLE)


def reload_class_labels_cache() -> None:
    """Clear cache (tests or forced reload)."""
    global _LABELS_CACHE, _LABELS_MTIME
    _LABELS_CACHE = None
    _LABELS_MTIME = None
