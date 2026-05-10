"""Seed module for py-public-api-004.

Change the raised exception type from ValueError to InputError.
"""


def parse_input(s: str) -> int:
    if not s:
        raise ValueError("empty input")
    return int(s)


def load_input(s: str) -> int:
    try:
        return parse_input(s)
    except ValueError:
        return -1


if __name__ == "__main__":
    print(load_input("42"))
