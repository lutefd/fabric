package main

import (
	"os"

	"github.com/lutefd/fabric/internal/cli"
)

func main() {
	mainWithExit(os.Args[1:], os.Exit)
}

func mainWithExit(args []string, exit func(int)) {
	if code := mainWithArgs(args); code != 0 {
		exit(code)
	}
}

func mainWithArgs(args []string) int {
	return cli.Execute(args, os.Stderr)
}
