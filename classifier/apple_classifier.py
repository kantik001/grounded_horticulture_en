import torch
import torch.nn as nn
from torchvision import models, transforms
from PIL import Image
import numpy as np
from typing import Tuple, Dict, Any
import json
import io
# import os
# from dotenv import load_dotenv


class AppleClassifier:
    # Class labels for apple-related categories
    CLASS_LABELS = [
        'healthy_apple',
        'apple_scab',
        'black_rot',
        'cedar_apple_rust',
        'healthy_leaf',
        'powdery_mildew',
        'fire_blight',
        'bitter_rot',
        'blue_mold',
        'brown_rot'
    ]

    def __init__(self, model_path: str = '../models/mobilenet_v2-b0353104.pth', num_classes: int = 10):
        """
        Initialize the classifier with MobileNetV2 model.

        Args:
            model_path: Path to pre-trained weights file (optional)
            num_classes: Number of classes for classification
        """

        # load_dotenv()

        self.device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
        self.num_classes = num_classes
        self.model = self._load_model(model_path)
        # self.model = self._load_model(os.getenv('MODEL_PATH'))
        self.model.to(self.device)
        self.model.eval()

        # Image preprocessing transforms
        self.transform = transforms.Compose([
            transforms.Resize((224, 224)),
            transforms.ToTensor(),
            transforms.Normalize(
                mean=[0.485, 0.456, 0.406],
                std=[0.229, 0.224, 0.225]
            )
        ])

    def _load_model(self, model_path: str = None) -> nn.Module:
        """
        Load MobileNetV2 model with custom classifier head.

        Args:
            model_path: Path to pre-trained weights

        Returns:
            Loaded PyTorch model
        """
        # Load pretrained MobileNetV2
        model = models.mobilenet_v2(weights=models.MobileNet_V2_Weights.IMAGENET1K_V1)

        # Replace the final classifier for our number of classes
        last_channel = model.classifier[1].in_features
        model.classifier = nn.Sequential(
            nn.Dropout(p=0.2),
            nn.Linear(last_channel, self.num_classes)
        )

        # Load custom weights if provided
        if model_path:
            checkpoint = torch.load(model_path, map_location=self.device)
            model.load_state_dict(checkpoint['state_dict'])

        return model

    def predict(self, image_path: str) -> Dict[str, Any]:
        """
        Predict class for a given image.

        Args:
            image_path: Path to the input image

        Returns:
            Dictionary containing prediction results
        """
        try:
            # Load and preprocess image
            image = Image.open(image_path).convert('RGB')
            image_tensor = self.transform(image).unsqueeze(0).to(self.device)

            # Run inference
            with torch.no_grad():
                outputs = self.model(image_tensor)
                probabilities = torch.softmax(outputs, dim=1)
                confidence, predicted = torch.max(probabilities, 1)

            # Get prediction details
            pred_idx = predicted.item()
            conf_score = confidence.item()
            pred_label = self.CLASS_LABELS[pred_idx]

            # Get top-3 predictions
            top_probs, top_indices = torch.topk(probabilities, min(3, len(self.CLASS_LABELS)))
            top_predictions = [
                {
                    'label': self.CLASS_LABELS[idx.item()],
                    'confidence': prob.item()
                }
                for idx, prob in zip(top_indices[0], top_probs[0])
            ]

            result = {
                'success': True,
                'prediction': pred_label,
                'confidence': conf_score,
                'top_predictions': top_predictions,
                'image_processed': True
            }

            return result

        except Exception as e:
            return {
                'success': False,
                'error': str(e),
                'image_processed': False
            }

    def predict_from_bytes(self, image_bytes: bytes) -> Dict[str, Any]:
        """
        Predict class from image bytes (for API usage).

        Args:
            image_bytes: Raw image bytes

        Returns:
            Dictionary containing prediction results
        """
        try:
            # Load image from bytes
            image = Image.open(io.BytesIO(image_bytes)).convert('RGB')
            image_tensor = self.transform(image).unsqueeze(0).to(self.device)

            # Run inference
            with torch.no_grad():
                outputs = self.model(image_tensor)
                probabilities = torch.softmax(outputs, dim=1)
                confidence, predicted = torch.max(probabilities, 1)

            # Get prediction details
            pred_idx = predicted.item()
            conf_score = confidence.item()
            pred_label = self.CLASS_LABELS[pred_idx]

            # Get top-3 predictions
            top_probs, top_indices = torch.topk(probabilities, min(3, len(self.CLASS_LABELS)))
            top_predictions = [
                {
                    'label': self.CLASS_LABELS[idx.item()],
                    'confidence': prob.item()
                }
                for idx, prob in zip(top_indices[0], top_probs[0])
            ]

            result = {
                'success': True,
                'prediction': pred_label,
                'confidence': conf_score,
                'top_predictions': top_predictions,
                'image_processed': True
            }

            return result

        except Exception as e:
            return {
                'success': False,
                'error': str(e),
                'image_processed': False
            }


def create_classifier(model_path: str = None) -> AppleClassifier:
    """
    Factory function to create an AppleClassifier instance.

    Args:
        model_path: Optional path to model weights

    Returns:
        Initialized AppleClassifier
    """
    return AppleClassifier(model_path=model_path)


if __name__ == '__main__':
    # Example usage
    import io

    classifier = create_classifier()

    # Test with an image file
    test_image = 'test_apple.jpg'
    result = classifier.predict(test_image)
    print(json.dumps(result, indent=2))