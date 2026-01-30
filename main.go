package main

import (
	"codeberg.org/goern/forgejo-mcp/v2/cmd"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

func main() {
	cmd.Execute(Version, Commit)
}
