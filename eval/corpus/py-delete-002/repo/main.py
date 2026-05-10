"""Seed module for py-delete-002.

Delete the legacy alias OldName.
"""


class NewName:
    def value(self) -> str:
        return "value"


OldName = NewName


def use_alias() -> str:
    return OldName().value()


if __name__ == "__main__":
    print(use_alias())
