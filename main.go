package main

import (
	"os"

	"github.com/lutefd/fabric/internal/cli"
)

func main() {
	if code := cli.Execute(os.Args[1:], os.Stderr); code != 0 {
		os.Exit(code)
	}
}
