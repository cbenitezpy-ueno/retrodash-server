// Package web serves the react-native-web bundle produced by the
// client/webpack.config.js build into this package's dist/ directory.
// The bundle is embedded into the Go binary at compile time via
// //go:embed — no runtime dependency on disk assets.
package web

import "embed"

// Assets holds the compiled web bundle produced by `npm run web:build`.
// The `dist/` directory MUST contain at least a `.gitkeep` file so that
// //go:embed compiles successfully before the first webpack run. The
// `all:` prefix is required to include dotfiles like `.gitkeep`.
//
//go:embed all:dist
var Assets embed.FS
