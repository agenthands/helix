package pkg

// Logger is a fixture logger with multiple methods.
type Logger struct {
	Prefix string
}

// Log emits a generic message.
func (l *Logger) Log(msg string) {
	_ = l.Prefix + ": " + msg
}

// Info emits info.
func (l *Logger) Info(msg string) {
	l.Log("info " + msg)
}

// Debug emits debug.
func (l *Logger) Debug(msg string) {
	l.Log("debug " + msg)
}

// Warn emits warn.
func (l *Logger) Warn(msg string) {
	l.Log("warn " + msg)
}

// Error emits error.
func (l *Logger) Error(msg string) {
	l.Log("error " + msg)
}
