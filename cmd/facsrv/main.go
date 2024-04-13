// Package main provides the "facsrv" executable.
//
// facsrv is a tool for managing a Factorio server installation.
//
// For managing server mods, see [github.com/nesv/factorio-tools/cmd/facmod].
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	ff "github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

func main() {
	rootFlags := ff.NewFlagSet("facsrv")

	rootCmd := &ff.Command{
		Name:        "facsrv",
		Usage:       "facsrv [FLAGS] COMMAND",
		ShortHelp:   "Manage your Factorio server installation",
		Flags:       rootFlags,
		Subcommands: []*ff.Command{},
	}

	if err := rootCmd.ParseAndRun(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd))
		if errors.Is(err, flag.ErrHelp) || errors.Is(err, ff.ErrNoExec) {
			return
		}
		fmt.Fprintln(os.Stderr, "error: ", err)
		os.Exit(1)
	}
}
