package main

import (
	"gitea.com/gitea/gitea-mcp/cmd"
)

var (
	Version = "dev"
)

func main() {
	cmd.Execute(Version)
}
