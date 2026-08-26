"""Safe, optional ONNX model loading for the image-classifier sidecar."""

from __future__ import annotations

from dataclasses import dataclass

import io
import os
import socket
import time
from typing import Tuple, Union
from urllib.parse import urlparse
from ipaddress import ip_address, IPv4Address, IPv6Address

import numpy as np
import requests
from PIL import Image
from requests.exceptions import InvalidURL


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
        ImageModel._validate_url(image_url)
        response = requests.get(image_url, timeout=5, allow_redirects=False)
        response.raise_for_status()
        return Image.open(io.BytesIO(response.content)).convert("RGB")

    @staticmethod
    def _validate_url(url: str) -> None:
        """Validate URL to prevent SSRF attacks including DNS rebinding."""
        try:
            parsed = urlparse(url)
        except (ValueError, InvalidURL):
            raise ValueError("Invalid URL format")

        if parsed.scheme not in ("http", "https"):
            raise ValueError(f"Scheme {parsed.scheme} not allowed, only http/https")

        hostname = parsed.hostname
        if not hostname:
            raise ValueError("URL missing hostname")

        hostname_lower = hostname.lower()
        if hostname_lower in ("localhost", "0.0.0.0", "127.0.0.1", "::1", "[::1]"):
            raise ValueError(f"Access to {hostname} not allowed")

        if hostname_lower.startswith("[") and hostname_lower.endswith("]"):
            ipv6_str = hostname_lower[1:-1]
            if ipv6_str in ("::1", "::"):
                raise ValueError(f"Access to {hostname} not allowed")

        if hostname.endswith((".local", ".internal", ".localhost")):
            raise ValueError("Access to local domains not allowed")

        try:
            ip = ip_address(hostname)
            ImageModel._check_ip_blocked(ip)
        except ValueError:
            try:
                ips = socket.getaddrinfo(hostname, None, socket.AF_UNSPEC, socket.SOCK_STREAM)
                if not ips:
                    raise ValueError("Hostname resolution returned no results")
                for family, socktype, proto, canonname, sockaddr in ips:
                    resolved_ip = ip_address(sockaddr[0])
                    ImageModel._check_ip_blocked(resolved_ip)
            except socket.gaierror as e:
                raise ValueError(f"Failed to resolve hostname: {e}")

    @staticmethod
    def _check_ip_blocked(ip: Union[IPv4Address, IPv6Address]) -> None:
        """Check if IP is in blocked ranges."""
        if ip.is_loopback:
            raise ValueError(f"Access to loopback addresses not allowed: {ip}")
        if ip.is_private:
            raise ValueError(f"Access to private IP ranges not allowed: {ip}")
        if ip.is_link_local:
            raise ValueError(f"Access to link-local addresses not allowed: {ip}")
        if ip.is_reserved:
            raise ValueError(f"Access to reserved addresses not allowed: {ip}")
        if ip.is_multicast:
            raise ValueError(f"Access to multicast addresses not allowed: {ip}")

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
