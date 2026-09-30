package templates

import "embed"

// FS contains embedded templates used by the generator.
//
//go:embed assets/**/*
var FS embed.FS
