"""Generic JSON-over-gRPC implementation of the inference.proto service."""

from __future__ import annotations

import json
from concurrent import futures

import grpc
from prometheus_client import Counter, Histogram

from model_loader import ImageModel

INFERENCE_LATENCY = Histogram("apartment_hunter_inference_latency_seconds", "Sidecar inference latency")
INFERENCE_ERRORS = Counter("apartment_hunter_inference_errors_total", "Sidecar inference errors")


def _decode(raw: bytes) -> dict:
    return json.loads(raw.decode("utf-8")) if raw else {}


def _encode(value: dict) -> bytes:
    return json.dumps(value).encode("utf-8")


class ImageClassifierService:
    def __init__(self, model: ImageModel) -> None:
        self.model = model

    def classify(self, request: dict, _context: grpc.ServicerContext) -> dict:
        result = self.model.predict(request.get("image_url", ""), request.get("image_path", ""))
        INFERENCE_LATENCY.observe(result.latency_ms / 1000)
        if result.error:
            INFERENCE_ERRORS.inc()
        return {"ai_score": result.ai_score, "confidence": result.confidence, "latency_ms": result.latency_ms, "error": result.error}

    @staticmethod
    def health(_request: dict, _context: grpc.ServicerContext) -> dict:
        return {"status": "SERVING"}

    def register(self, server: grpc.Server) -> None:
        classifier = grpc.unary_unary_rpc_method_handler(self.classify, request_deserializer=_decode, response_serializer=_encode)
        health = grpc.unary_unary_rpc_method_handler(self.health, request_deserializer=_decode, response_serializer=_encode)
        server.add_generic_rpc_handlers((
            grpc.method_handlers_generic_handler("inference.ImageClassifier", {"ClassifyAIGenerated": classifier}),
            grpc.method_handlers_generic_handler("grpc.health.v1.Health", {"Check": health}),
        ))


def create_server(model: ImageModel) -> grpc.Server:
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=4))
    ImageClassifierService(model).register(server)
    return server
