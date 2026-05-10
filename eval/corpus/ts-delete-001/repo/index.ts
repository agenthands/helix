// legacyFormat formats a value using the old format. @deprecated Use newFormat instead.
export function legacyFormat(value: number): string {
  return `VALUE: ${value}`;
}

// newFormat formats a value using the current standard.
export function newFormat(value: number): string {
  return JSON.stringify({ value });
}

export function printValue(n: number): void {
  console.log(newFormat(n));
}
