package main

import (
	"codeberg.org/goern/forgejo-mcp/v2/cmd"
)

var Version = "dev"

func main() {
	cmd.Execute(Version)
}
