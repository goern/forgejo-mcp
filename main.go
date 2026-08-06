package main

import (
	"runtime/debug"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/cmd"
)

var Version = "dev"

func main() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
	cmd.Execute(Version)
}
