// DEFAULT_TIMEOUT is the exported default timeout constant.
export const DEFAULT_TIMEOUT = 5000;

// withTimeout calls DEFAULT_TIMEOUT.
export function withTimeout(custom?: number): number {
  return custom ?? DEFAULT_TIMEOUT;
}
