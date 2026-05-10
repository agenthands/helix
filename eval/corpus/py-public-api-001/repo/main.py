"""Batch processor — add timeout_sec parameter to process_batch."""
from typing import List


def process_batch(items: List[str]) -> List[str]:
    """Process a batch of items and return results.

    TODO: add optional timeout_sec parameter (default=30).
    """
    return [item.strip().upper() for item in items]


def run_pipeline(items: List[str]) -> None:
    """Entry point that calls process_batch."""
    results = process_batch(items)
    for r in results:
        print(r)


if __name__ == "__main__":
    run_pipeline(["alpha", "beta", "gamma"])
