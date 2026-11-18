// Package dashcrew embeds the neutral catalog (core/, platforms/, packs/) so the binary carries it.
package dashcrew

import "embed"

// CatalogFS holds core/, platforms/ and packs/ at their repository paths.
// The all: prefix keeps dotfiles: the catalog loader reads every file under these folders.
//
//go:embed all:core all:platforms all:packs
var CatalogFS embed.FS

// PersonalFS holds personal/: the personal instruction template and the personal skills `dashcrew me sync` writes
// outside every repo. They are never part of a project's generated toolkit.
//
//go:embed all:personal
var PersonalFS embed.FS
