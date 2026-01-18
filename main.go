// Package main is the entry point for wedevctl CLI.
package main

import (
	"fmt"
	"os"

	cmd "github.com/wedevctl/cmd"
)

func main() {
	root := cmd.NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
