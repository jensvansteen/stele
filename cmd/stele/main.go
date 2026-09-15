package main

import (
	"os"

	"github.com/jensvansteen/stele/internal/stele"
)

func main() {
	os.Exit(stele.Run(os.Args[1:], os.Stdout, os.Stderr))
}
