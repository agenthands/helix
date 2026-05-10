interface Session {
  id: string;
  userId: string;
  expiresAt: number;
}

// createSession initialises a new session for the given user.
// TODO: change return type to Promise<Session> (async session store).
export function createSession(userId: string): Session {
  return {
    id: `sess-${Math.random().toString(36).slice(2)}`,
    userId,
    expiresAt: Date.now() + 3600_000,
  };
}

export function startUserSession(userId: string): Session {
  return createSession(userId);
}
