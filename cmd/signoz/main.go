package main

import (
	"os"

	"github.com/swtormy-agents/signoz-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}
