package main

import (
	"forgejo.com/forgejo/forgejo-mcp/cmd"
)

var (
	Version = "dev"
)

func main() {
	cmd.Execute(Version)
}
