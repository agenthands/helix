// loadConfig loads a config and returns a string. Will be widened to return a Config object.
export function loadConfig(name: string): string {
  return name;
}

// bootstrap calls loadConfig.
export function bootstrap(name: string): string {
  const cfg = loadConfig(name);
  return "loaded:" + cfg;
}
