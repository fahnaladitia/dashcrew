// Package dashcrew embeds the neutral catalog (core/, platforms/, packs/) so the binary carries it.
package dashcrew

import "embed"

// CatalogFS holds core/, platforms/ and packs/ at their repository paths.
// The all: prefix keeps dotfiles: the catalog loader reads every file under these folders.
//
//go:embed all:core all:platforms all:packs
var CatalogFS embed.FS

// StartersFS holds starters/: platform starters that `dashcrew platform new` copies into a project's
// `.ai/platforms/<name>/`. They are never part of the catalog itself.
//
//go:embed all:starters
var StartersFS embed.FS
