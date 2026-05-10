"""Seed module for py-public-api-003.

Change the default value of bonus from 0 to 10.
"""


def compute_score(base: int, bonus: int = 0) -> int:
    return base + bonus


def report_score(base: int) -> str:
    return f"score={compute_score(base)}"


if __name__ == "__main__":
    print(report_score(5))
