package main

import (
	"context"
	"os"

	"github.com/coterm/coterm/internal/cli"
)

func main() {
	code := cli.Main(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
