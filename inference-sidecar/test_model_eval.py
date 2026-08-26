from model_eval import compute_metrics


def test_metrics_are_correct():
    result = compute_metrics([0.9, 0.8, 0.2, 0.1], [1, 0, 1, 0])
    assert result["precision"] == 0.5
    assert result["recall"] == 0.5
    assert result["total_test_samples"] == 4
