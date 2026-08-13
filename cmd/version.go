package cmd

// version is the mininote CLI version, stamped at build time via
//
//	go build -ldflags "-X github.com/dakolli/mininote-cli/cmd.version=<ver>"
//
// The default "dev" marks local/unreleased builds; release builds get the tag
// (e.g. v1.2.3) from GoReleaser.
var version = "dev"
