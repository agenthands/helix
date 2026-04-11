class Config:
    """Configuration for the worker service."""

    def __init__(self, name: str) -> None:
        self.name = name


def Handler(c: Config) -> str:
    """Process a Config and return its name."""
    return c.name


def Parse(raw: str) -> Config:
    """Create a Config from a raw string."""
    return Config(name=raw)
