import io
import json
from typing import Any, Dict

import torch
import torch.nn as nn
from PIL import Image
from torchvision import models, transforms


class AppleClassifier:
    # Метки классов (индекс выхода модели = позиция в списке).
    CLASS_LABELS = [
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

    # Инициализирует MobileNetV2, загружает веса и задаёт препроцессинг 224×224.
    def __init__(
        self,
        model_path: str = "../models/mobilenet_v2-b0353104.pth",
        num_classes: int = 10,
    ):
        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        self.num_classes = num_classes
        self.model = self._load_model(model_path)
        self.model.to(self.device)
        self.model.eval()
        # Нормализация как у ImageNet (mean/std).
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

    # Собирает MobileNetV2 с головой на num_classes и подгружает checkpoint при наличии.
    def _load_model(self, model_path: str = None) -> nn.Module:
        model = models.mobilenet_v2(weights=models.MobileNet_V2_Weights.IMAGENET1K_V1)
        last_channel = model.classifier[1].in_features
        model.classifier = nn.Sequential(
            nn.Dropout(p=0.2),
            nn.Linear(last_channel, self.num_classes),
        )
        if model_path:
            checkpoint = torch.load(model_path, map_location=self.device)
            model.load_state_dict(checkpoint["state_dict"])
        return model

    # Классифицирует изображение по пути к файлу; возвращает label, confidence, top-3.
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

    # Классифицирует изображение из байтов (для HTTP API Flask).
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

    def _run_inference(self, image_tensor: torch.Tensor) -> Dict[str, Any]:
        with torch.no_grad():
            outputs = self.model(image_tensor)
            probabilities = torch.softmax(outputs, dim=1)
            confidence, predicted = torch.max(probabilities, 1)

        pred_idx = predicted.item()
        top_probs, top_indices = torch.topk(
            probabilities, min(3, len(self.CLASS_LABELS))
        )
        top_predictions = [
            {
                "label": self.CLASS_LABELS[idx.item()],
                "confidence": prob.item(),
            }
            for idx, prob in zip(top_indices[0], top_probs[0])
        ]

        return {
            "success": True,
            "prediction": self.CLASS_LABELS[pred_idx],
            "confidence": confidence.item(),
            "top_predictions": top_predictions,
            "image_processed": True,
        }


# Фабрика: создаёт экземпляр AppleClassifier с опциональным путём к весам.
def create_classifier(model_path: str = None) -> AppleClassifier:
    return AppleClassifier(model_path=model_path)


if __name__ == "__main__":
    classifier = create_classifier()
    result = classifier.predict("test_apple.jpg")
    print(json.dumps(result, indent=2))
