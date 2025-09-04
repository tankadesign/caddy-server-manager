package main

import (
	"os"

	"github.com/tankadesign/caddy-site-manager/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
