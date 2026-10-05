// Package version reports the CFGB release identity.
package version

// Version is the executable version and the source of truth for releases.
// Bump it in a PR to release that version when the PR is merged to main.
// GoReleaser also sets it from the matching release tag via -X.
var Version = "v0.1.0"
