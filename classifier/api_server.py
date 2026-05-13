"""
Python-сервис: классификация изображений (MobileNetV2) и RAG-retrieval (контекст для Go).
"""

from flask import Flask, request, jsonify
from flask_cors import CORS
import sys
import os

_root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
sys.path.insert(0, _root)

from dotenv import load_dotenv

load_dotenv(os.path.join(_root, ".env"))

from classifier.apple_classifier import create_classifier

app = Flask(__name__)
CORS(app)

classifier = None


def get_classifier():
    global classifier
    if classifier is None:
        model_path = os.environ.get("MODEL_PATH")
        if model_path and not os.path.isabs(model_path):
            model_path = os.path.normpath(os.path.join(os.path.dirname(__file__), model_path))
        if model_path and os.path.exists(model_path):
            print(f"Loading model from: {model_path}")
            classifier = create_classifier(model_path=model_path)
        else:
            print("No model file found. Starting with ImageNet weights only.")
            print("Set MODEL_PATH environment variable to load custom weights.")
            classifier = create_classifier()
    return classifier


@app.route("/classify", methods=["POST"])
def classify_image():
    try:
        if "image" not in request.files:
            return jsonify({"success": False, "error": "No image file provided"}), 400

        file = request.files["image"]
        if file.filename == "":
            return jsonify({"success": False, "error": "Empty filename"}), 400

        image_bytes = file.read()
        if len(image_bytes) == 0:
            return jsonify({"success": False, "error": "Empty image file"}), 400

        clf = get_classifier()
        result = clf.predict_from_bytes(image_bytes)
        response = jsonify(result)
        response.headers.set("Content-Type", "application/json; charset=utf-8")
        return response, 200

    except Exception as e:
        return jsonify({"success": False, "error": str(e)}), 500


@app.route("/rag/context", methods=["POST"])
def rag_context():
    """Поиск по статьям: возвращает контекст и few-shot для сборки промпта в Go."""
    try:
        data = request.get_json(silent=True) or {}
        question = (data.get("question") or "").strip()
        if not question:
            return jsonify({"success": False, "error": "Пустой вопрос"}), 400

        from rag.retrieval import retrieve_rag_context

        payload = retrieve_rag_context(question)
        resp = jsonify(payload)
        resp.headers.set("Content-Type", "application/json; charset=utf-8")
        return resp, 200
    except Exception as e:
        return jsonify({"success": False, "error": str(e)}), 500


@app.route("/health", methods=["GET"])
def health_check():
    return jsonify({"status": "healthy", "service": "apple-python"}), 200


if __name__ == "__main__":
    port = int(os.environ.get("CLASSIFIER_PORT", 5000))
    print(f"Starting Apple Python API on port {port}")
    app.run(host="0.0.0.0", port=port, debug=False)
