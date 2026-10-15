package graph

import "embed"

// WebAssets holds the embedded static files for the local relationship graph viewer.
//
//go:embed web/*
var WebAssets embed.FS
