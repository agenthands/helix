package daemon

import (
	// Blank imports trigger skill.Register() via init() (Caddy-style).
	_ "github.com/postfix/serena/internal/kernel/diag"
	_ "github.com/postfix/serena/internal/kernel/edit"
	_ "github.com/postfix/serena/internal/kernel/fileops"
	_ "github.com/postfix/serena/internal/kernel/symbols"
	_ "github.com/postfix/serena/internal/profile"
	_ "github.com/postfix/serena/internal/skill/memory"
	_ "github.com/postfix/serena/internal/skill/repomap"
	_ "github.com/postfix/serena/internal/skill/workflow"
)
