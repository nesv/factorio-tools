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
	"log"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	semver "github.com/Masterminds/semver/v3"
	humanize "github.com/dustin/go-humanize"
	ff "github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/nesv/factorio-tools/mods"
	"github.com/nesv/factorio-tools/userdata"
)

func main() {
	log.SetPrefix("facmod: ")
	log.SetFlags(0)

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

	searchFlags := ff.NewFlagSet("search").SetParent(rootFlags)
	searchFlags.BoolVar(&searchSortByDate, 't', "sort-by-date", "Sort results by release date")
	searchFlags.StringEnumVar(&searchCategory, 'c', "category", "Only show mods in the given category", mods.Categories()...)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "facmod search [FLAGS] SEARCH_TERM",
		ShortHelp: "Search the local mod cache",
		Flags:     searchFlags,
		Exec:      runSearch,
	}

	categoriesFlags := ff.NewFlagSet("categories").SetParent(rootFlags)
	categoriesCmd := &ff.Command{
		Name:      "categories",
		Usage:     "facmod categories",
		ShortHelp: "List all available mod categories",
		Flags:     categoriesFlags,
		Exec:      runCategories,
	}

	installFlags := ff.NewFlagSet("install").SetParent(rootFlags)
	installFlags.BoolVar(&installOptional, 'o', "optional", "Install optional dependencies")
	installFlags.BoolVar(&installEnable, 'e', "enable", "Enable mods after installation")
	installCmd := &ff.Command{
		Name:      "install",
		Usage:     "facmod install [FLAGS] MOD ...",
		ShortHelp: "Install mods",
		Flags:     installFlags,
		Exec:      runInstall,
	}

	root := &ff.Command{
		Name:      "facmod",
		Usage:     "facmod [FLAGS] SUBCOMMAND ...",
		ShortHelp: "Factorio server mod manager",
		Flags:     rootFlags,
		Subcommands: []*ff.Command{
			categoriesCmd,
			cleanCmd,
			installCmd,
			listCmd,
			searchCmd,
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

// Set by command-line flags.
var (
	searchSortByDate bool
	searchCategory   string
)

func runSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("at least one search term is required")
	}

	cacheDir, err := makeCacheDir()
	if err != nil {
		return fmt.Errorf("make cache dir: %w", err)
	}

	cache, err := mods.OpenCache(cacheDir)
	if err != nil {
		return fmt.Errorf("open cache: %w", err)
	}
	defer cache.Close()

	var options []mods.SearchOption
	if searchSortByDate {
		options = append(options, mods.SortByDate())
	}
	if searchCategory != "" {
		c := mods.Category(searchCategory)
		options = append(options, mods.WithCategories(c))
	}

	mm, err := cache.Search(ctx, args[0], options...)
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 8, 1, ' ', 0)
	defer tw.Flush()

	headers := []string{"NAME", "CATEGORY", "VERSION", "RELEASED", "SUMMARY"}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))

	for _, m := range mm {
		relt := humanize.Time(m.ReleasedAt)
		summary := m.Summary
		if len(summary) > 30 {
			summary = summary[0:30] + "..."
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			m.Name,
			m.Category,
			m.Versions[0],
			relt,
			summary,
		)
	}

	return nil
}

func runCategories(ctx context.Context, args []string) error {
	for _, c := range mods.Categories() {
		if c == "" {
			continue
		}
		fmt.Println(c)
	}
	return nil
}

// Set by command-line flags for the "facmod install" command.
var (
	installOptional bool
	installEnable   bool
)

// runInstall is the entrypoint for the "facmod install" command.
func runInstall(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("at least one mod name is required")
	}

	// Load the player data to get the username and token.
	playerData, err := userdata.LoadPlayerData(installDir)
	if err != nil {
		return fmt.Errorf("load player data: %w", err)
	}
	if playerData.ServiceUsername == "" {
		return errors.New("service-username is not set in player data")
	}
	if playerData.ServiceToken == "" {
		return errors.New("service-token is not set in player data")
	}

	// Open the cache.
	cacheDir, err := makeCacheDir()
	if err != nil {
		return fmt.Errorf("make cache dir: %w", err)
	}

	cache, err := mods.OpenCache(cacheDir)
	if err != nil {
		return fmt.Errorf("open cache: %w", err)
	}
	defer cache.Close()

	// Collect all of the mods that are already cached, and already
	// installed, and see which ones we need to download.
	// cached, err := cache.Mods()
	// if err != nil {
	// 	return fmt.Errorf("list cached mods: %w", err)
	// }
	//
	// installation, err := server.Open(installDir)
	// if err != nil {
	// 	return fmt.Errorf("open installation dir: %w", err)
	// }
	//
	// installed, err := installation.Mods()
	// if err != nil {
	// 	return fmt.Errorf("list installed mods: %w", err)
	// }

	// Get the download URL and version for all of the specified mods.
	mm := make([]minimod, len(args))
	for i, modName := range args {
		downloadURL, err := cache.DownloadURL(ctx, modName)
		if err != nil {
			return fmt.Errorf("get download url for %q: %w", modName, err)
		}

		version, err := cache.LatestVersion(ctx, modName)
		if err != nil {
			return fmt.Errorf("get latest version for %q: %w", modName, err)
		}

		mm[i] = minimod{
			name:    modName,
			url:     downloadURL,
			version: version,
		}
	}
	slices.SortFunc(mm, func(a, b minimod) int {
		if a.name < b.name {
			return -1
		} else if a.name > b.name {
			return 1
		}
		if a.version.LessThan(b.version) {
			return -1
		} else if a.version.GreaterThan(b.version) {
			return 1
		}
		return 0
	})

	toInstall := make(map[string]string) // name -> cached path
	for _, m := range mm {
		log.Printf("download %s_%s", m.name, m.version)
		cachedPath, err := cache.Get(ctx, m.name, playerData.ServiceUsername, playerData.ServiceToken)
		if err != nil {
			return fmt.Errorf("get %s_%s: %w", m.name, m.version, err)
		}

		toInstall[m.name] = cachedPath

		// Fetch all of the mod's dependencies.
		info, err := mods.LoadInfo(cachedPath)
		if err != nil {
			return fmt.Errorf("load mod info: %w", err)
		}

		deps, err := info.Dependencies()
		if err != nil {
			return fmt.Errorf("get dependencies: %w", err)
		}

		// Install all required dependencies.
		for i, d := range deps.Required {
			if d.Name == "base" {
				// The "base" mod is provided by the
				// installation.
				continue
			}

			leader := "\u251c"
			if i == len(deps.Required)-1 && len(deps.Optional) > 0 && !installOptional {
				leader = "\u2514"
			}
			log.Println(leader, d)

			cachedPath, err := cache.Get(ctx,
				d.Name,
				playerData.ServiceUsername,
				playerData.ServiceToken,
			)
			if err != nil {
				return fmt.Errorf("get %s ", d)
			}
			toInstall[d.Name] = cachedPath
		}

		// Install optional dependencies?
		if installOptional {
			for i, d := range deps.Optional {
				leader := "\u251c"
				if i == len(deps.Optional)-1 {
					leader = "\u2514"
				}
				log.Println(leader, d)

				cachedPath, err := cache.Get(ctx,
					d.Name,
					playerData.ServiceUsername,
					playerData.ServiceToken,
				)
				if err != nil {
					return fmt.Errorf("get %s_%s", d.Name, d.Version)
				}
				toInstall[d.Name] = cachedPath
			}
		}
	}

	// Install the cached mods.
	for name, cachedPath := range toInstall {
		log.Println("install", name)

		installPath := filepath.Join(installDir, "mods", filepath.Base(cachedPath))
		if _, err := os.Stat(installPath); errors.Is(err, fs.ErrNotExist) {
			log.Printf("copy %s -> %s\n", cachedPath, installPath)
		}
	}

	// TODO: Update mod-list.json.
	if installEnable {
		log.Println("updating mod-list.json")
	}

	return nil
}

type minimod struct {
	name    string
	url     *url.URL
	version *semver.Version
}
