package pkg

import "strings"

// Trim strips surrounding whitespace; uses stdlib.
func Trim(s string) string {
	return strings.TrimSpace(s)
}

// Split cuts s by sep.
func Split(s, sep string) []string {
	return strings.Split(s, sep)
}

// LogAll logs every entry in items via the given Logger.
func LogAll(l *Logger, items []string) {
	for _, it := range items {
		l.Log(Trim(it))
	}
}
