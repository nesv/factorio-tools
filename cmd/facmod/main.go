// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main provides the facmod executable, for helping you manage mods on
// your Factorio server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	ff "github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"git.sr.ht/~nesv/factorio-tools/mods"
)

func main() {
	rootFlags := ff.NewFlagSet("facmod")
	rootFlags.StringVar(&installDir, 'D', "directory", "/opt/factorio", "Path to the Factorio installation directory")
	rootFlags.BoolVar(&noHeaders, 'H', "no-headers", "Disable headers on tabular output")

	cleanFlags := ff.NewFlagSet("clean").SetParent(rootFlags)
	cleanCmd := &ff.Command{
		Name:      "clean",
		Usage:     "facmod clean",
		ShortHelp: "Clean the cache",
		Flags:     cleanFlags,
		Exec:      runClean,
	}

	listFlags := ff.NewFlagSet("list").SetParent(rootFlags)
	listCmd := &ff.Command{
		Name:      "list",
		Usage:     "facmod list [--installed] [FLAGS]",
		ShortHelp: "List mods",
		Flags:     listFlags,
		Exec:      runList,
	}

	updateFlags := ff.NewFlagSet("update").SetParent(rootFlags)
	updateCmd := &ff.Command{
		Name:      "update",
		Usage:     "facmod update [FLAGS]",
		ShortHelp: "Update the local mod cache",
		Flags:     updateFlags,
		Exec:      runUpdate,
	}

	root := &ff.Command{
		Name:      "facmod",
		Usage:     "facmod [FLAGS] SUBCOMMAND ...",
		ShortHelp: "Factorio server mod manager",
		Flags:     rootFlags,
		Subcommands: []*ff.Command{
			cleanCmd,
			listCmd,
			updateCmd,
		},
	}
	if err := root.ParseAndRun(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(root))
		if errors.Is(err, flag.ErrHelp) || errors.Is(err, ff.ErrNoExec) {
			return
		}
		fmt.Fprintln(os.Stderr, "error: ", err)
		os.Exit(1)
	}
}

// Set by command-line flags.
var (
	installDir string
	noHeaders  bool
)

// runUpdate is the entrypoint for the "update" subcommand.
func runUpdate(ctx context.Context, args []string) error {
	// Fetch all pages from the mod portal, and write them to the cache dir.
	cacheDir, err := makeCacheDir()
	if err != nil {
		return fmt.Errorf("make cache dir: %w", err)
	}

	cache, err := mods.OpenCache(cacheDir)
	if err != nil {
		return fmt.Errorf("open cache: %w", err)
	}
	defer cache.Close()
	cache.EnableProgressBar()

	if err := cache.Pull(ctx); err != nil {
		return fmt.Errorf("pull latest mod list: %w", err)
	}

	if err := cache.Update(ctx); err != nil {
		return fmt.Errorf("update cache: %w", err)
	}

	return nil
}

func makeCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("user cache dir: %w", err)
	}

	dir = filepath.Join(dir, "facmod")
	if err := os.MkdirAll(dir, fs.ModePerm); err != nil {
		return "", fmt.Errorf("make directory %q: %w", dir, err)
	}

	return dir, nil
}

// runClean is the entrypoint for the "clean" subcommand.
func runClean(ctx context.Context, args []string) error {
	cacheDir, err := makeCacheDir()
	if err != nil {
		return fmt.Errorf("make cache dir: %w", err)
	}

	cache, err := mods.OpenCache(cacheDir)
	if err != nil {
		return fmt.Errorf("open cache: %w", err)
	}
	defer cache.Close()

	if err := cache.Clean(); err != nil {
		return err
	}

	return cache.Clean()
}

// runList is the entrypoint for the "list" subcommand.
func runList(ctx context.Context, args []string) error {
	mm, err := mods.Load(installDir)
	if err != nil {
		return fmt.Errorf("load mods: %w", err)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 8, 1, ' ', 0)
	defer tw.Flush()

	if !noHeaders {
		header := []string{
			"NAME",
			"VERSION",
			"ENABLED",
		}
		fmt.Fprintln(tw, strings.Join(header, "\t"))
	}

	for _, m := range mm {
		var latestVersion mods.Version
		if n := len(m.Versions); n != 0 {
			latestVersion = m.Versions[n-1]
		}
		fmt.Fprintf(tw, "%s\t%s\t%t\n", m.Name, latestVersion, m.Enabled)
	}

	return nil
}
