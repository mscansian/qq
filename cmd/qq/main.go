package main

import (
	"os"

	"github.com/mscansian/qq/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
