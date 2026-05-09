package help

import "embed"

//go:embed docs/*.md
var EmbeddedTopicDocs embed.FS
