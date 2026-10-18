// Command hebrew-mcp serves the hebrew-lessons MCP endpoint on /mcp.
package main

import (
	"fmt"
	"os"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/mcp"
)

func main() {
	if err := mcp.Listen(); err != nil {
		fmt.Fprintf(os.Stderr, "hebrew-mcp: %v\n", err)
		os.Exit(1)
	}
}
