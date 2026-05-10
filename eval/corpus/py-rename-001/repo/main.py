"""Auth utilities — rename validate_token to check_auth_token throughout."""


def validate_token(token: str) -> bool:
    """Return True if the token is non-empty and well-formed."""
    return bool(token) and token.startswith("Bearer ")


def authenticate(token: str) -> str:
    """Return the auth result for the given token."""
    if validate_token(token):
        return "authenticated"
    return "rejected"


if __name__ == "__main__":
    print(authenticate("Bearer secret"))
