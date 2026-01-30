package main

import (
	"os"

	"sprite-bootstrap/cmd"
)

// version is set by the linker at build time
var version = "dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
