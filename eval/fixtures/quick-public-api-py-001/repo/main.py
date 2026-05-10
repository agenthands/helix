from __future__ import annotations


def compute(x: int) -> tuple[int, str]:
    """Compute a result from x, returning (value, status)."""
    return x * 2, "ok"


if __name__ == "__main__":
    result, status = compute(21)
    print(result, status)
