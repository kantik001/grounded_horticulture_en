import io
import json
from typing import Any, Dict, List, Optional

import torch
import torch.nn as nn
from PIL import Image
from torchvision import models, transforms

from cv.labels_config import default_class_labels_for_crop

# Apple labels from config/cv_class_labels.json (when checkpoint has no class_labels).
DEFAULT_CLASS_LABELS = default_class_labels_for_crop("apple")


class AppleClassifier:
    """Apple disease classifier on MobileNetV2 (inference from file or bytes)."""

    # Initializes device, class labels, loads weights, and sets 224x224 transforms.
    def __init__(
        self,
        model_path: Optional[str] = None,
        num_classes: int = 10,
    ):
        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        self.class_labels: List[str] = list(DEFAULT_CLASS_LABELS)
        self.num_classes = num_classes
        checkpoint = self._read_checkpoint(model_path)
        if checkpoint:
            labels = checkpoint.get("class_labels")
            if isinstance(labels, list) and labels:
                self.class_labels = [str(x) for x in labels]
                self.num_classes = len(self.class_labels)
        self.model = self._build_model(checkpoint)
        self.model.to(self.device)
        self.model.eval()
        self.transform = transforms.Compose(
            [
                transforms.Resize((224, 224)),
                transforms.ToTensor(),
                transforms.Normalize(
                    mean=[0.485, 0.456, 0.406],
                    std=[0.229, 0.224, 0.225],
                ),
            ]
        )

    # Reads a .pth checkpoint from disk; returns None on error or missing path.
    def _read_checkpoint(self, model_path: Optional[str]) -> Optional[dict]:
        if not model_path:
            return None
        try:
            return torch.load(
                model_path,
                map_location=self.device,
                weights_only=False,
            )
        except Exception:
            return None

    # Builds MobileNetV2 with num_classes head and loads checkpoint state_dict when present.
    def _build_model(self, checkpoint: Optional[dict]) -> nn.Module:
        model = models.mobilenet_v2(weights=models.MobileNet_V2_Weights.IMAGENET1K_V1)
        last_channel = model.classifier[1].in_features
        model.classifier = nn.Sequential(
            nn.Dropout(p=0.2),
            nn.Linear(last_channel, self.num_classes),
        )
        if checkpoint and "state_dict" in checkpoint:
            model.load_state_dict(checkpoint["state_dict"])
        return model

    # Classifies an image by file path; returns success/prediction/confidence dict.
    def predict(self, image_path: str) -> Dict[str, Any]:
        try:
            image = Image.open(image_path).convert("RGB")
            image_tensor = self.transform(image).unsqueeze(0).to(self.device)
            return self._run_inference(image_tensor)
        except Exception as e:
            return {
                "success": False,
                "error": str(e),
                "image_processed": False,
            }

    # Classifies image bytes (HTTP multipart); same response shape as predict.
    def predict_from_bytes(self, image_bytes: bytes) -> Dict[str, Any]:
        try:
            image = Image.open(io.BytesIO(image_bytes)).convert("RGB")
            image_tensor = self.transform(image).unsqueeze(0).to(self.device)
            return self._run_inference(image_tensor)
        except Exception as e:
            return {
                "success": False,
                "error": str(e),
                "image_processed": False,
            }

    # Runs tensor through model: softmax, best class, and top-3 with confidences.
    def _run_inference(self, image_tensor: torch.Tensor) -> Dict[str, Any]:
        with torch.no_grad():
            outputs = self.model(image_tensor)
            probabilities = torch.softmax(outputs, dim=1)
            confidence, predicted = torch.max(probabilities, 1)

        pred_idx = predicted.item()
        if pred_idx >= len(self.class_labels):
            return {
                "success": False,
                "error": f"class index {pred_idx} out of label range",
                "image_processed": False,
            }
        top_probs, top_indices = torch.topk(
            probabilities, min(3, len(self.class_labels))
        )
        top_predictions = [
            {
                "label": self.class_labels[idx.item()],
                "confidence": prob.item(),
            }
            for idx, prob in zip(top_indices[0], top_probs[0])
            if idx.item() < len(self.class_labels)
        ]

        return {
            "success": True,
            "prediction": self.class_labels[pred_idx],
            "confidence": confidence.item(),
            "top_predictions": top_predictions,
            "image_processed": True,
        }


# Factory: creates AppleClassifier with optional path to trained .pth weights.
def create_classifier(model_path: str = None) -> AppleClassifier:
    return AppleClassifier(model_path=model_path)


if __name__ == "__main__":
    classifier = create_classifier()
    result = classifier.predict("test_apple.jpg")
    print(json.dumps(result, indent=2))
