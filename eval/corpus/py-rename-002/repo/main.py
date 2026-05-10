"""Seed module for py-rename-002.

Rename DataLoader class to DataReader.
"""


class DataLoader:
    def __init__(self, source: str) -> None:
        self.source = source

    def read(self) -> str:
        return f"data from {self.source}"


def load_all(sources: list[str]) -> list[str]:
    return [DataLoader(s).read() for s in sources]


if __name__ == "__main__":
    print(load_all(["a", "b"]))
