"""Seed module for py-delete-001.

Delete the unused legacy helper.
"""


def _legacy_helper(s: str) -> str:
    return f"[{s}]"


def current_helper(s: str) -> str:
    return s + "!"


if __name__ == "__main__":
    print(current_helper("hi"))
