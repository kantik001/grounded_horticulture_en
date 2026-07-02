"""CV model factory by crop_id."""

import os
from typing import Dict, Optional

from cv.apple_classifier import create_classifier
from rag.crops_config import get_crop, normalize_crop_id

_classifiers: Dict[str, object] = {}


# Returns .pth path from MODEL_PATH or MODEL_PATH_{CROP} for the crop.
def _model_path_for_crop(crop_id: str) -> Optional[str]:
    env_key = f"MODEL_PATH_{crop_id.upper()}"
    path = os.environ.get(env_key)
    if path:
        return path
    if crop_id == "apple":
        return os.environ.get("MODEL_PATH")
    return None


# Returns (creates and caches) a classifier for crop_id when cv_enabled in crops.json.
def get_classifier_for_crop(crop_id: str):
    crop_id = normalize_crop_id(crop_id)
    crop = get_crop(crop_id)
    if not crop.get("cv_enabled", False):
        raise ValueError(
            f"Photo recognition for «{crop.get('name_en', crop.get('name_ru', crop_id))}» is not available yet."
        )

    if crop_id in _classifiers:
        return _classifiers[crop_id]

    model_path = _model_path_for_crop(crop_id)
    if model_path and not os.path.isabs(model_path):
        model_path = os.path.normpath(os.path.join(os.path.dirname(__file__), model_path))

    if model_path and os.path.exists(model_path):
        print(f"[CV:{crop_id}] Loading weights: {model_path}")
        clf = create_classifier(model_path=model_path)
    else:
        print(f"[CV:{crop_id}] No weights — ImageNet backbone only.")
        clf = create_classifier()

    _classifiers[crop_id] = clf
    return clf
