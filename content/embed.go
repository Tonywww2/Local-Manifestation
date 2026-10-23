// Package bundled embeds the original data; the embedding wrapper is outside the content package directory.
package bundled

import (
	"embed"
	"io/fs"
)

//go:embed base/manifest.json base/definitions.json base/adventure.json base/behavior.js base/assets/art-v1
var files embed.FS

var Files = func() fs.FS {
	sub, err := fs.Sub(files, "base")
	if err != nil {
		panic(err)
	}
	return sub
}()
