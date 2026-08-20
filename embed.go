// Package macwtf exposes the embedded tool catalogue.
//
// The manifest and profile TOML files are baked into the binary so that a
// single downloaded executable is fully self-contained. Development and
// contributor workflows override this with --manifest-dir or MACWTF_MANIFEST_DIR,
// which reads the same files from a working checkout instead.
package macwtf

import "embed"

//go:embed manifest/*.toml profiles/*.toml
var Catalogue embed.FS
