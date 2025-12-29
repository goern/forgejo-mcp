package main

import (
	"codeberg.org/goern/forgejo-mcp/cmd"
)

var (
	Version = "dev"
)

func main() {
	cmd.Execute(Version)
}
