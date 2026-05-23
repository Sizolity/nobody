package main

import (
	"context"
	"os"

	"github.com/sizolity/nobody/internal/world/devcli"
)

func main() {
	os.Exit(devcli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
