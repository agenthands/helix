// computeTotal computes a total. Will accept a new options parameter.
export function computeTotal(items: number[]): number {
  return items.reduce((a, b) => a + b, 0);
}

// summarize calls computeTotal.
export function summarize(items: number[]): string {
  const total = computeTotal(items);
  return "total=" + total;
}
