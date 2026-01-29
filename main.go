package main

import (
	"os"

	"sprite-bootstrap/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
