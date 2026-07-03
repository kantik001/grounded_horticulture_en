# Walkthrough: `cv/apple_classifier.py`

**Source file:** `cv/apple_classifier.py`  
**Language:** Python (PyTorch + torchvision)  
**Related modules:** `cv/registry.py`, `api/app.py`, `cv/train_classifier.py`  
**Called by:** `registry.get_classifier_for_crop()` → `app.py` (`POST /classify`)

---

## Why this file exists

**Neural network for apple/leaf disease and condition recognition from photos**.

- Architecture: **MobileNetV2** (light CNN, suitable for mobile/server).
- Output: **one of 10 classes** + confidence + top-3 variants.
- Two image inputs: **file path** (`predict`) or **HTTP bytes** (`predict_from_bytes`).

File does **not** serve HTTP — only model logic. HTTP is in `app.py`.

---

## Classes the model recognizes

Default labels — from **`config/cv_class_labels.json`** (`cv/labels_config.py`), export **`DEFAULT_CLASS_LABELS`** for apple. On `.pth` load list may be replaced by **`class_labels` from checkpoint**:

| Index | Label | Meaning (brief) |
|-------|--------|-----------------|
| 0 | `healthy_apple` | Healthy apple |
| 1 | `apple_scab` | Scab |
| 2 | `black_rot` | Black rot |
| 3 | `cedar_apple_rust` | Cedar apple rust |
| 4 | `healthy_leaf` | Healthy leaf |
| 5 | `powdery_mildew` | Powdery mildew |
| 6 | `fire_blight` | Fire blight |
| 7 | `bitter_rot` | Bitter rot |
| 8 | `blue_mold` | Blue mold |
| 9 | `brown_rot` | Brown rot |

During training (`train_classifier.py`) dataset folders follow **`DEFAULT_CLASS_LABELS`** order; checkpoint saves `class_labels` — inference uses same order.

---

## Initialization: `__init__` (lines 28–54)

```python
AppleClassifier(model_path='../models/mobilenet_v2-b0353104.pth', num_classes=10)
```

What happens:

1. **`self.device`** — `cuda` if GPU available, else `cpu`.
2. Reads checkpoint (if any) → **`class_labels`** and **`num_classes`**.
3. **`self.model = self._build_model(checkpoint)`** — backbone + head + `state_dict`.
4. **`self.model.eval()`** — inference mode (no training dropout, fixed behavior).
5. **`self.transform`** — image pipeline:
   - resize **224×224** (ImageNet/MobileNet standard);
   - to tensor;
   - **Normalize** with ImageNet mean/std — same as backbone pretraining.

Without your weights model still “runs”, but head is random → predictions meaningless until you train and set `MODEL_PATH`.

---

## Model load: `_read_checkpoint` + `_build_model`

### Step 1 — backbone

```python
model = models.mobilenet_v2(weights=models.MobileNet_V2_Weights.IMAGENET1K_V1)
```

MobileNetV2 pretrained on ImageNet (general features: shapes, textures, edges).

### Step 2 — custom classifier head

Last layer replaced with:

- `Dropout(0.2)` — regularization during training;
- `Linear(..., num_classes)` — **10 outputs** (disease/condition count).

Transfer learning: ImageNet body, last layer for your classes.

### Step 3 — your weights (if `model_path` exists)

Checkpoint read **once** in `__init__`; `_build_model` applies `state_dict`.

Expected `.pth` format (as saved by `train_classifier.py`):

```python
{
    'epoch': ...,
    'state_dict': model.state_dict(),
    'class_labels': [...],  # labels from training dataset
    'val_acc': ...
}
```

In `registry.py` weight path comes from `.env` (`MODEL_PATH`, `MODEL_PATH_{CROP}`). If file missing — log: “No weights — ImageNet backbone only”.

---

## Inference: shared logic

`predict` and `predict_from_bytes` do the same thing; only image source differs:

| Method | Input |
|--------|-------|
| `predict(image_path)` | file path on disk |
| `predict_from_bytes(image_bytes)` | bytes from multipart (API) |

### Pipeline (same)

1. **PIL** opens image, `convert('RGB')` (even if grayscale/RGBA).
2. **`self.transform(image)`** → tensor 1×3×224×224.
3. **`torch.no_grad()`** — no gradients, faster, less memory.
4. **`outputs = self.model(image_tensor)`** — raw logits (10 numbers).
5. **`softmax`** → class probabilities.
6. **`torch.max`** → highest-probability class and **confidence**.
7. **`torch.topk(..., 3)`** → top three for UI/debug.

### Success response (dict)

```json
{
  "success": true,
  "prediction": "apple_scab",
  "confidence": 0.87,
  "top_predictions": [
    {"label": "apple_scab", "confidence": 0.87},
    {"label": "powdery_mildew", "confidence": 0.08},
    {"label": "healthy_leaf", "confidence": 0.03}
  ],
  "image_processed": true
}
```

### Error

```json
{
  "success": false,
  "error": "exception description",
  "image_processed": false
}
```

Exceptions are not re-raised — API gets JSON with `success: false` (convenient for Go).

---

## Factory: `create_classifier` (lines 190–200)

```python
def create_classifier(model_path: str = None) -> AppleClassifier:
    return AppleClassifier(model_path=model_path)
```

Used in **`registry.py`**: create classifier once per crop and cache in `_classifiers[crop_id]`.

---

## `if __name__ == '__main__'` block (lines 203–212)

Local test without server:

```bash
cd cv
python apple_classifier.py
```

Expects `test_apple.jpg` nearby — manual check after training.

---

## System integration

```mermaid
flowchart LR
    A[api/app.py POST /classify] --> B[registry.get_classifier_for_crop]
    B --> C[create_classifier + MODEL_PATH]
    C --> D[AppleClassifier.predict_from_bytes]
    D --> E[JSON prediction + confidence]
    E --> F[Go sendToClassifier]
    F --> G[LLM or template recommendation]
```

### Go after Python

In Go (`server/classifier_client.go`, `server/photo_recommendations.go`):

- parses JSON into `ClassificationResult` (`prediction`, `confidence`, `top_predictions`);
- `classifyAndRecommend` → `generatePhotoRecommendation` / `generateTemplateRecommendation` — advice text by class;
- saves `class_prediction`, `class_confidence` in DB (`postgres_store.go`).

Confidence threshold for “not sure” is still planned in roadmap (phase 4) — `apple_classifier.py` has **no** threshold cutoff yet, always returns best class.

---

## Model training (separate file)

Weights for `_load_model` come from **`train_classifier.py`**:

- dataset: folders per class (`healthy_apple/`, `apple_scab/`, …);
- same MobileNetV2 architecture + head replacement;
- save to `.pth` with `state_dict` key.

After training:

1. Put file e.g. `models/apple_classifier.pth`.
2. In `.env`: `MODEL_PATH=models/apple_classifier.pth` (or docker volume path).
3. Restart `classifier` container.

Training walkthrough — [cv-train_classifier.md](./cv-train_classifier.md).

---

## FAQ

### Why 224×224 and these mean/std?

MobileNetV2 and ImageNet trained at this size and normalization. Different numbers → worse quality without retraining.

### Why `num_classes=10` and 10 labels in list?

Must match. Adding a class — change list, `num_classes`, and retrain.

### Model always confident but wrong

Likely no your `.pth` or too little training data. Check registry log: `Loading weights: ...` vs `No weights — ImageNet backbone only`.

### `predict` and `predict_from_bytes`

Both call shared **`_run_inference`** after tensor preprocessing.

---

## What to read next

| Topic | File |
|-------|------|
| HTTP and `/classify` | [python-api.md](./python-api.md) |
| Model selection and cache | [cv-registry.md](./cv-registry.md) |
| Training weights | [cv-train_classifier.md](./cv-train_classifier.md) |
| User recommendation after CV | `server/classify_flow.go`, `photo_recommendations.go` (`generatePhotoRecommendation`) |

---

## Brief summary

`apple_classifier.py` — **CV core**: load MobileNetV2, replace head for 10 classes, preprocessing, inference, JSON result. In production called only via **`predict_from_bytes`** from `app.py`; Go gets `prediction` and `confidence` and adds text.
