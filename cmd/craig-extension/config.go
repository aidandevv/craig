package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aidandevv/craig-extension/internal/config"
)

const configUsage = `Usage:
  craig-extension config init [--config PATH] [--vision-api-key KEY]
  craig-extension config token [--config PATH]

config init creates a private configuration file, starter rules.yaml, and a
random connection code. Google Vision is optional: leave --vision-api-key unset to
use the ${GOOGLE_VISION_API_KEY} reference in the generated configuration.

After setup, start the Craig helper with craig-extension daemon. The
extension finds a helper on the default local address automatically; the token
is a one-time connection code for the extension options page.
`

func runConfig(args []string, stdout io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, configUsage)
		return exitUsage
	}
	switch args[0] {
	case "init":
		return runConfigInit(args[1:], stdout)
	case "token":
		return runConfigToken(args[1:], stdout)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, configUsage)
		return exitOK
	default:
		fmt.Fprintf(stdout, "unknown config command %q\n\n%s", args[0], configUsage)
		return exitUsage
	}
}

func runConfigInit(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("config init", flag.ContinueOnError)
	fs.SetOutput(stdout)
	path := fs.String("config", "", "configuration file path")
	visionKey := fs.String("vision-api-key", "", "Google Vision API key (optional; saved in private config)")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return exitUsage
	}
	if *path == "" {
		defaultPath, err := config.DefaultPath()
		if err != nil {
			fmt.Fprintf(stdout, "error: %v\n", err)
			return exitRuntime
		}
		*path = defaultPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stdout, "error: find home directory: %v\n", err)
		return exitRuntime
	}
	cfg, err := config.Default(home)
	if err != nil {
		fmt.Fprintf(stdout, "error: %v\n", err)
		return exitRuntime
	}
	// A deliberately chosen config location should keep its editable rules next
	// to it. The conventional default remains ~/.config/craig-extension.
	if *path != config.DefaultPaths(home).Config {
		cfg.Rules.File = filepath.Join(filepath.Dir(*path), "rules.yaml")
	}
	if strings.TrimSpace(*visionKey) != "" {
		cfg.APIKeys.GoogleVision = *visionKey
	}
	if err := config.Initialize(*path, cfg); err != nil {
		fmt.Fprintf(stdout, "error: %v\n", err)
		return exitRuntime
	}
	fmt.Fprintf(stdout, "Created private config: %s\nCreated starter rules: %s\n", *path, cfg.Rules.File)
	fmt.Fprintln(stdout, "Google Vision is optional. You can add its API key from the Craig extension options after the helper starts.")
	fmt.Fprintln(stdout, "When the extension asks for a connection code, paste this token:")
	fmt.Fprintln(stdout, cfg.Daemon.Token)
	return exitOK
}

func runConfigToken(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("config token", flag.ContinueOnError)
	fs.SetOutput(stdout)
	path := fs.String("config", "", "configuration file path")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return exitUsage
	}
	if *path == "" {
		defaultPath, err := config.DefaultPath()
		if err != nil {
			fmt.Fprintf(stdout, "error: %v\n", err)
			return exitRuntime
		}
		*path = defaultPath
	}
	cfg, err := config.Load(*path)
	if err != nil {
		fmt.Fprintf(stdout, "error: %v\n", err)
		return exitRuntime
	}
	fmt.Fprintln(stdout, cfg.Daemon.Token)
	return exitOK
}
