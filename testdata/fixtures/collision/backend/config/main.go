package config

// Config holds configuration for the backend service.
type Config struct {
	Name string
}

// Handler processes a Config and returns its name.
func Handler(c Config) string {
	return c.Name
}

// Parse creates a Config from a raw string.
func Parse(raw string) Config {
	return Config{Name: raw}
}
