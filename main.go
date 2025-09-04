package main

import (
	"os"

	"github.com/falcon/caddy-site-manager/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
