// currentHelper is actively used.
export function currentHelper(): string {
  return "active";
}

// legacyHelper is no longer referenced anywhere.
export function legacyHelper(): string {
  return "legacy";
}

console.log(currentHelper());
