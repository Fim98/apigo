// Package version holds the version string of the command-line interface.
//
// Release builds set Value with -ldflags "-X apigo/internal/version.Value=x.y.z".
// The desktop app reads its version from MyGo (mygo.App.Version), which
// mygo.json supplies, so this package has a single source of truth per binary.
package version

// Value is the version reported by apigo-cli.
var Value = "dev"
