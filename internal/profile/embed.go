package profile

import "embed"

//go:embed profiles/*.yaml
var embeddedProfiles embed.FS

//go:embed modes/*.yaml
var embeddedModes embed.FS
