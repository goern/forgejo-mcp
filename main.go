package main

import (
	"forgejo.org/forgejo/forgejo-mcp/cmd"
)

var (
	Version = "dev"
)

func main() {
	cmd.Execute(Version)
}
