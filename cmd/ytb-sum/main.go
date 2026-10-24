// Command ytb-sum summarizes YouTube videos in the terminal.
package main

import (
	"os"

	"github.com/antalkon/ytb-sum/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
