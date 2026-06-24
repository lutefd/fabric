package main

import (
	"fmt"
	"os"

	"github.com/lutefd/fabric/internal/fabric"
)

func main() {
	if err := fabric.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fabric:", err)
		os.Exit(1)
	}
}
