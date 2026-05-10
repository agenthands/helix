"""Seed module for py-rename-003.

Rename the module-level constant MAX_BUFFER to MAX_BUFFER_BYTES.
"""


MAX_BUFFER = 4096


def read_chunk(stream) -> bytes:
    return stream.read(MAX_BUFFER)


if __name__ == "__main__":
    print(read_chunk(_StubStream()))
