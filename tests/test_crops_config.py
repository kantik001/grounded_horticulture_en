"""Unit tests for config/crops.json."""

import os

import pytest

from rag.crops_config import get_crop, list_crops, normalize_crop_id


@pytest.fixture(autouse=True)
def crops_config_path(monkeypatch):
    """Point CROPS_CONFIG_PATH at the repo config and reset the module cache."""
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    monkeypatch.setenv("CROPS_CONFIG_PATH", os.path.join(root, "config", "crops.json"))
    import rag.crops_config as cc

    cc._CONFIG = None
    cc._CONFIG_MTIME = None


def test_normalize_crop_id_apple():
    """normalize_crop_id trims whitespace and lowercases a known crop."""
    assert normalize_crop_id("apple") == "apple"
    assert normalize_crop_id(" Apple ") == "apple"


def test_normalize_crop_id_unknown():
    """Unknown crop id raises ValueError."""
    with pytest.raises(ValueError, match="Unknown crop"):
        normalize_crop_id("banana_xyz")


def test_list_crops_has_apple():
    """list_crops exposes apple as the RAG-enabled default crop."""
    data = list_crops()
    assert data["default_crop"] == "apple"
    assert "apple" in data["crops"]
    assert get_crop("apple").get("rag_enabled") is True


def test_demo_hr_sandbox_domain():
    """Platform generality: RAG without CV."""
    hr = get_crop("demo_hr")
    assert hr.get("rag_enabled") is True
    assert hr.get("cv_enabled") is False
