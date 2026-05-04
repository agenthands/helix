from typing import TypeVar

T = TypeVar("T")
U = TypeVar("U")

def map_list(xs: list[T], f) -> list[U]:
    return [f(x) for x in xs]
