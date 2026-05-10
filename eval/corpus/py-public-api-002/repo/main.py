"""Seed module for py-public-api-002.

Add a required user_id keyword parameter.
"""


def fetch_user(name: str) -> dict[str, str]:
    return {"name": name}


def summarize_user(name: str) -> str:
    user = fetch_user(name)
    return f"user={user['name']}"


if __name__ == "__main__":
    print(summarize_user("alice"))
