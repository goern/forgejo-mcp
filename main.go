package main

import (
	"runtime/debug"

	"codeberg.org/goern/forgejo-mcp/v2/cmd"
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
