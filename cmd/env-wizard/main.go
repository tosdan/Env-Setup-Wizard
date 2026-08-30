package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = ""
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(versionText())
		return
	}

	fmt.Fprintln(os.Stderr, "env-wizard: wizard implementation not available yet")
	os.Exit(1)
}

func versionText() string {
	if commit == "" {
		return "env-wizard " + version
	}

	return fmt.Sprintf("env-wizard %s (commit %s)", version, commit)
}
