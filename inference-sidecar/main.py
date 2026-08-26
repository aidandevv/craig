from __future__ import annotations

import logging
import os
import threading
import time

import requests
from prometheus_client import Gauge, start_http_server

from inference_service import create_server
from model_eval import evaluate_dataset
from model_loader import ImageModel

MODEL_AVAILABLE = Gauge("apartment_hunter_model_available", "Whether the mounted ONNX model loaded successfully")
MODEL_PRECISION = Gauge("apartment_hunter_model_precision", "Held-out model precision", ["model_version"])
MODEL_RECALL = Gauge("apartment_hunter_model_recall", "Held-out model recall", ["model_version"])
MODEL_F1 = Gauge("apartment_hunter_model_f1", "Held-out model F1", ["model_version"])
MODEL_FPR = Gauge("apartment_hunter_model_false_positive_rate", "Held-out model false-positive rate", ["model_version"])
MODEL_FNR = Gauge("apartment_hunter_model_false_negative_rate", "Held-out model false-negative rate", ["model_version"])


def publish_evaluation(model: ImageModel) -> None:
    dataset = os.getenv("TEST_SET_PATH", "/app/test-data/held-out.json")
    result = evaluate_dataset(model, dataset)
    if not result:
        return
    version = os.getenv("MODEL_VERSION", "sentry-image-v1")
    MODEL_PRECISION.labels(version).set(result["precision"])
    MODEL_RECALL.labels(version).set(result["recall"])
    MODEL_F1.labels(version).set(result["f1"])
    MODEL_FPR.labels(version).set(result["false_positive_rate"])
    MODEL_FNR.labels(version).set(result["false_negative_rate"])
    core_url = os.getenv("CORE_API_URL", "")
    if core_url:
        payload = dict(result)
        payload["model_version"] = version
        try:
            requests.post(core_url.rstrip("/") + "/api/model-evaluations", json=payload, timeout=5).raise_for_status()
        except requests.RequestException:
            logging.exception("could not persist model evaluation through Go API")


def weekly_evaluator(model: ImageModel) -> None:
    interval_seconds = int(os.getenv("MODEL_EVALUATION_INTERVAL_SECONDS", str(7 * 24 * 60 * 60)))
    while True:
        publish_evaluation(model)
        time.sleep(max(interval_seconds, 60))


def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))
    model = ImageModel()
    MODEL_AVAILABLE.set(1 if model.available else 0)
    start_http_server(int(os.getenv("METRICS_PORT", "9091")))
    threading.Thread(target=weekly_evaluator, args=(model,), daemon=True).start()
    server = create_server(model)
    server.add_insecure_port("[::]:" + os.getenv("GRPC_PORT", "50051"))
    server.start()
    logging.info("inference sidecar started; model available=%s", model.available)
    server.wait_for_termination()


if __name__ == "__main__":
    main()
