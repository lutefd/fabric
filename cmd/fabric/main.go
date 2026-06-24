package main

import (
	"fmt"
	"os"

	"github.com/lutefd/fabric/internal/fabric"
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
	if err := fabric.Run(args); err != nil {
		fmt.Fprintln(os.Stderr, "fabric:", err)
		return 1
	}
	return 0
}
