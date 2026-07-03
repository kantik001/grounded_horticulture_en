"""
Python HTTP API: CV (/classify) and RAG retrieval (/rag/context) for the Go server.
"""

import hmac
import os
import sys

_root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
sys.path.insert(0, _root)

from dotenv import load_dotenv
from flask import Flask, jsonify, request
from flask_cors import CORS

load_dotenv(os.path.join(_root, ".env"))

from cv.registry import ModelWeightsUnavailableError, get_classifier_for_crop
from rag.crops_config import list_crops, normalize_crop_id
from rag.retrieval import retrieve_rag_context
from rag import vector_store as vs
from rag.warmup import warmup_rag

# Upload cap for /classify photos; Flask rejects larger requests with 413.
MAX_UPLOAD_BYTES = int(os.environ.get("MAX_UPLOAD_BYTES", str(10 * 1024 * 1024)))

app = Flask(__name__)
app.config["MAX_CONTENT_LENGTH"] = MAX_UPLOAD_BYTES + 512 * 1024  # multipart overhead
_cors_origins = [
    o.strip()
    for o in os.environ.get(
        "CORS_ALLOWED_ORIGINS",
        "http://localhost,http://127.0.0.1",
    ).split(",")
    if o.strip()
]
CORS(app, origins=_cors_origins, supports_credentials=True)


@app.route("/classify", methods=["POST"])
def classify_image():
    """POST /classify: classify a plant photo for the given crop_id."""
    try:
        crop_id = request.form.get("crop_id") or request.args.get("crop_id") or "apple"
        try:
            crop_id = normalize_crop_id(crop_id)
        except ValueError as e:
            return jsonify({"success": False, "error": str(e)}), 400

        if "image" not in request.files:
            return jsonify({"success": False, "error": "No image file provided"}), 400

        file = request.files["image"]
        if file.filename == "":
            return jsonify({"success": False, "error": "Empty filename"}), 400

        image_bytes = file.read(MAX_UPLOAD_BYTES + 1)
        if len(image_bytes) == 0:
            return jsonify({"success": False, "error": "Empty image file"}), 400
        if len(image_bytes) > MAX_UPLOAD_BYTES:
            return (
                jsonify(
                    {
                        "success": False,
                        "error": f"Image too large (max {MAX_UPLOAD_BYTES // (1024 * 1024)} MB)",
                    }
                ),
                413,
            )

        clf = get_classifier_for_crop(crop_id)
        result = clf.predict_from_bytes(image_bytes)
        result["crop_id"] = crop_id
        response = jsonify(result)
        response.headers.set("Content-Type", "application/json; charset=utf-8")
        return response, 200

    except ValueError as e:
        return jsonify({"success": False, "error": str(e)}), 400
    except ModelWeightsUnavailableError as e:
        return jsonify({"success": False, "error": str(e)}), 503
    except Exception:
        app.logger.exception("classify failed")
        return jsonify({"success": False, "error": "Internal error"}), 500


@app.route("/rag/context", methods=["POST"])
def rag_context():
    """POST /rag/context: retrieve RAG context fragments for a question."""
    try:
        data = request.get_json(silent=True) or {}
        question = (data.get("question") or "").strip()
        crop_id = data.get("crop_id") or "apple"
        if not question:
            return jsonify({"success": False, "error": "Empty question"}), 400

        payload = retrieve_rag_context(question, crop_id=crop_id)
        resp = jsonify(payload)
        resp.headers.set("Content-Type", "application/json; charset=utf-8")
        status = 200 if payload.get("success") else 422
        return resp, status
    except Exception:
        app.logger.exception("rag context failed")
        return jsonify({"success": False, "error": "Internal error"}), 500


@app.route("/crops", methods=["GET"])
def crops_list():
    """GET /crops: list supported crops."""
    return jsonify({"success": True, **list_crops()}), 200


@app.route("/health", methods=["GET"])
def health_check():
    """GET /health: liveness check."""
    return jsonify({"status": "healthy", "service": "garden-python"}), 200


@app.route("/admin/reindex", methods=["POST"])
def admin_reindex():
    """POST /admin/reindex: rebuild the RAG index (requires X-Admin-Secret)."""
    expected = os.environ.get("ADMIN_SECRET", "")
    secret = request.headers.get("X-Admin-Secret", "")
    if not expected or not hmac.compare_digest(secret, expected):
        return jsonify({"success": False, "error": "forbidden"}), 403
    try:
        vs.reset_vector_store()
        store = vs.load_vector_store(force_reindex=True)
        if store is None:
            return jsonify({"success": False, "error": "No articles to index"}), 400
        return jsonify({"success": True, "message": "RAG reindexed"}), 200
    except Exception as e:
        return jsonify({"success": False, "error": str(e)}), 500


if __name__ == "__main__":
    port = int(os.environ.get("CLASSIFIER_PORT", 5000))
    if os.environ.get("USE_GUNICORN", "").lower() in ("1", "true", "yes"):
        import subprocess

        conf = os.path.join(os.path.dirname(__file__), "gunicorn.conf.py")
        print(f"Starting Python API (gunicorn) on port {port}")
        raise SystemExit(subprocess.call(["gunicorn", "-c", conf, "api.app:app"]))

    warmup_rag()
    print(f"Starting Python API (Flask dev) on port {port}")
    app.run(host="0.0.0.0", port=port, debug=False, threaded=True)
