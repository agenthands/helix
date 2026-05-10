// unusedExport is an unused legacy export.
export function unusedExport(x: number): number {
  return x * 2;
}

// activeExport calls unusedExport.
export function activeExport(x: number): number {
  return x + 1;
}
