from utils import greet


def helper() -> str:
    """A simple helper function for edit tests."""
    return "hello"


class DemoClass:
    def __init__(self, value: int) -> None:
        self.value = value

    def get_value(self) -> int:
        return self.value


def unused_func() -> None:
    """An unreferenced function for safe_delete tests."""
    pass


def using_helper() -> str:
    return helper()
