// deadBranchHandler is a dead branch handler that is never reached.
export function deadBranchHandler(x: number): string {
  return "dead:" + x;
}

// dispatch calls deadBranchHandler.
export function dispatch(x: number, useDead: boolean): string {
  if (useDead) {
    return deadBranchHandler(x);
  }
  return "live:" + x;
}
