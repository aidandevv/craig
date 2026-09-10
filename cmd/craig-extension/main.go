// Command craig-extension analyzes marketplace listings for scam indicators.
package main

import (
	"fmt"
	"os"
)

const usage = `craig-extension — scam indicators for marketplace listings

Usage:
  craig-extension analyze [flags]   analyze one listing

Run "craig-extension analyze -h" for flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(exitUsage)
	}
	switch os.Args[1] {
	case "analyze":
		os.Exit(runAnalyze(os.Args[2:], os.Stdin, os.Stdout))
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(exitUsage)
	}
}
