def greet(name: str) -> str:
    """Greet someone by name."""
    return f"Hello, {name}!"


class Greeter:
    """Interface-like class for cross-file reference tests."""

    def greet(self, name: str) -> str:
        return greet(name)
