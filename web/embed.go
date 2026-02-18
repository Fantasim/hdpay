package web

import "embed"

// StaticFiles embeds the SvelteKit static build output.
// The "all:" prefix ensures dotfiles and underscore-prefixed dirs (like _app/) are included.
//
//go:embed all:build
var StaticFiles embed.FS
