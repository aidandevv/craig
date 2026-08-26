"""Safe, optional ONNX model loading for the image-classifier sidecar."""

from __future__ import annotations

from dataclasses import dataclass

import io
import os
import time
from typing import Tuple

import numpy as np
import requests
from PIL import Image


@dataclass
class Prediction:
    ai_score: float
    confidence: float
    latency_ms: int
    error: str = ""


class ImageModel:
    """Loads a mounted ONNX model without downloading arbitrary model code.

    Model acquisition is a deployment decision. A missing model returns an
    explicit unavailable result rather than inventing an AI-image score.
    """

    def __init__(self, model_path: str | None = None) -> None:
        self.model_path = model_path or os.getenv("ONNX_MODEL_PATH", "/app/models/sentry-image.onnx")
        self.session = None
        self.load_error = ""
        try:
            if not os.path.isfile(self.model_path):
                self.load_error = f"model not found: {self.model_path}"
                return
            import onnxruntime as ort

            self.session = ort.InferenceSession(self.model_path, providers=["CPUExecutionProvider"])
        except Exception as exc:  # surfaced in the response and metric
            self.load_error = f"model load failed: {exc}"

    @property
    def available(self) -> bool:
        return self.session is not None

    def predict(self, image_url: str = "", image_path: str = "") -> Prediction:
        started = time.perf_counter()
        if not self.available:
            return Prediction(0.0, 0.0, elapsed_ms(started), self.load_error)
        try:
            image = self._load_image(image_url, image_path)
            tensor = self._preprocess(image)
            output = self.session.run(None, {self.session.get_inputs()[0].name: tensor})[0]
            score, confidence = self._score(np.asarray(output))
            return Prediction(score, confidence, elapsed_ms(started))
        except Exception as exc:
            return Prediction(0.0, 0.0, elapsed_ms(started), str(exc))

    @staticmethod
    def _load_image(image_url: str, image_path: str) -> Image.Image:
        if image_path:
            return Image.open(image_path).convert("RGB")
        if not image_url:
            raise ValueError("image_url or image_path is required")
        response = requests.get(image_url, timeout=5)
        response.raise_for_status()
        return Image.open(io.BytesIO(response.content)).convert("RGB")

    @staticmethod
    def _preprocess(image: Image.Image) -> np.ndarray:
        # Most image-classification ONNX exports accept normalized NCHW 224px
        # tensors. If a chosen model differs, its adapter belongs here.
        array = np.asarray(image.resize((224, 224)), dtype=np.float32) / 255.0
        array = (array - np.array([0.485, 0.456, 0.406], dtype=np.float32)) / np.array([0.229, 0.224, 0.225], dtype=np.float32)
        return np.transpose(array, (2, 0, 1))[None, ...]

    @staticmethod
    def _score(output: np.ndarray) -> Tuple[float, float]:
        values = output.reshape(-1).astype(float)
        if values.size == 1:
            probability = 1.0 / (1.0 + np.exp(-values[0]))
            return float(probability), float(max(probability, 1 - probability))
        shifted = values - values.max()
        probabilities = np.exp(shifted) / np.exp(shifted).sum()
        # The supported model must document the AI-generated class at index 1.
        ai_index = min(1, len(probabilities) - 1)
        return float(probabilities[ai_index]), float(probabilities.max())


def elapsed_ms(started: float) -> int:
    return int((time.perf_counter() - started) * 1000)
