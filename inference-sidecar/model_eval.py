"""Weekly held-out-set evaluation owned by the sidecar, never SQLite."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Iterable

from model_loader import ImageModel


def compute_metrics(predictions: Iterable[float], labels: Iterable[int], threshold: float = 0.5) -> dict[str, float | int]:
    pairs = [(prediction >= threshold, bool(label)) for prediction, label in zip(predictions, labels)]
    tp = sum(predicted and actual for predicted, actual in pairs)
    fp = sum(predicted and not actual for predicted, actual in pairs)
    fn = sum(not predicted and actual for predicted, actual in pairs)
    tn = sum(not predicted and not actual for predicted, actual in pairs)
    precision = tp / (tp + fp) if tp + fp else 0.0
    recall = tp / (tp + fn) if tp + fn else 0.0
    return {
        "precision": precision,
        "recall": recall,
        "f1": (2 * precision * recall / (precision + recall)) if precision + recall else 0.0,
        "false_positive_rate": fp / (fp + tn) if fp + tn else 0.0,
        "false_negative_rate": fn / (fn + tp) if fn + tp else 0.0,
        "total_test_samples": len(pairs),
    }


def evaluate_dataset(model: ImageModel, dataset_path: str) -> dict[str, float | int] | None:
    path = Path(dataset_path)
    if not model.available or not path.is_file():
        return None
    records = json.loads(path.read_text())
    predictions, labels = [], []
    for record in records:
        result = model.predict(image_url=record.get("image_url", ""), image_path=record.get("image_path", ""))
        if not result.error:
            predictions.append(result.ai_score)
            labels.append(int(record["label"]))
    return compute_metrics(predictions, labels) if predictions else None
