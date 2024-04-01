// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mods

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/Masterminds/squirrel"
	progressbar "github.com/schollz/progressbar/v3"
	_ "modernc.org/sqlite"

	"github.com/nesv/factorio-tools/httputil"
)

// Cache is a local database that is used for caching information about Factorio mods.
type Cache struct {
	dir string
	db  *sql.DB

	mu                sync.Mutex
	cachedResultsPath string
	showProgressBar   bool
}

func OpenCache(dir string) (*Cache, error) {
	dbPath := filepath.Join(dir, "mods.db")

	// If the database does not already exist, we will need to initialize it.
	var initp bool
	info, err := os.Stat(dbPath)
	if errors.Is(err, fs.ErrNotExist) {
		initp = true
	} else if err != nil {
		return nil, fmt.Errorf("stat %q: %w", dbPath, err)
	} else if err == nil && info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", dbPath)
	}

	db, err := sql.Open("sqlite", filepath.Join(dir, "mods.db"))
	if err != nil {
		return nil, fmt.Errorf("open mods.db: %w", err)
	}

	if initp {
		if err := initCacheDB(db); err != nil {
			return nil, fmt.Errorf("initialize cache database: %w", err)
		}
	}

	// SQLite does not currently enforce foreign keys automatically, and
	// we need to enable a pragma to have it do so.
	if _, err := db.Exec(`PRAGMA foriegn_keys = ON`); err != nil {
		return nil, fmt.Errorf("enable foreign_keys pragma: %w", err)
	}

	c := &Cache{
		dir: dir,
		db:  db,
	}

	return c, nil
}

func initCacheDB(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS categories (name TEXT PRIMARY KEY) STRICT`,
		`CREATE TABLE IF NOT EXISTS mods (name TEXT PRIMARY KEY, title TEXT, owner TEXT, summary TEXT, category TEXT REFERENCES categories(name)) STRICT`,
		`CREATE TABLE IF NOT EXISTS latest_releases (name TEXT PRIMARY KEY, download_url TEXT, file_name TEXT, info_json TEXT, released_at TEXT, version TEXT, sha1 TEXT) STRICT`,
	}

	for i, s := range statements {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("execute statement %d: %w", i+1, err)
		}
	}

	return nil
}

func (c *Cache) Close() error {
	return c.db.Close()
}

// EnableProgressBar prints a progress bar to STDERR for methods like [Cache.Pull],
// and [Cache.Update].
func (c *Cache) EnableProgressBar() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.showProgressBar = true
}

func (c *Cache) DisableProgressBar() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.showProgressBar = false
}

// Pull retrieves the mod list from the [Mods portal API], and caches the results,
// returning the path to the file holding the partially-processed results.
// The file holding the results contains a stream of mod entries, with each
// entry being its own JSON object.
// Use [encoding/json.Decoder] to read this file.
//
// To update the cache database, call [Cache.Update] afterwards.
func (c *Cache) Pull(ctx context.Context) error {
	resp, err := httputil.Get(ctx, "https://mods.factorio.com/api/mods")
	if err != nil {
		return fmt.Errorf("get first page: %w", err)
	}
	defer resp.Body.Close()

	var list modlist
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	results, err := c.makeTempFile("results.json")
	if err != nil {
		return fmt.Errorf("make temp file: %w", err)
	}
	defer results.Close()

	var (
		enc        = json.NewEncoder(results)
		totalPages = list.Pagination.PageCount

		showProgress = c.progressBarEnabled()
		bar          *progressbar.ProgressBar
	)

	if showProgress {
		bar = progressbar.NewOptions(totalPages,
			progressbar.OptionShowCount(),
			progressbar.OptionSetPredictTime(false),
			progressbar.OptionSetElapsedTime(true),
			progressbar.OptionSetDescription("Pulling mod list"),
			progressbar.OptionSetWriter(os.Stderr),
		)
		bar.Add(1)
		defer bar.Exit()
	}

	for i := 2; i <= totalPages; i++ {
		urlStr := fmt.Sprintf("https://mods.factorio.com/api/mods?page=%d", i)
		resp, err := httputil.Get(ctx, urlStr)
		if err != nil {
			return fmt.Errorf("http get %q: %w", urlStr, err)
		}

		// NOTE: resp.Body does not need to be closed, since it will be
		// done by decodeResults.

		mods, err := c.decodeResults(resp.Body)
		if err != nil {
			return fmt.Errorf("decode results for page %d: %w", i, err)
		}

		for _, m := range mods {
			if err := enc.Encode(m); err != nil {
				return fmt.Errorf("encode mod: %w", err)
			}
		}

		if showProgress {
			bar.Add(1)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedResultsPath = results.Name()

	return nil
}

func (c *Cache) progressBarEnabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.showProgressBar
}

func (c *Cache) decodeResults(r io.ReadCloser) ([]modlistResult, error) {
	defer r.Close()
	var list modlist
	if err := json.NewDecoder(r).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	return list.Results, nil
}

// makeTempFile creates a new file with name in a directory created by [os.MkdirTemp].
// The caller is responsible for deleting the file and its parent directory.
func (c *Cache) makeTempFile(name string) (*os.File, error) {
	dir, err := os.MkdirTemp(c.dir, "facmod-*")
	if err != nil {
		return nil, fmt.Errorf("make temp dir: %w", err)
	}

	return os.Create(filepath.Join(dir, name))
}

func (c *Cache) withLock(fn func() error) error {
	if fn == nil {
		return errors.New("nil func for lock")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return fn()
}

func (c *Cache) Update(ctx context.Context) error {
	var (
		showProgress = c.progressBarEnabled()
		bar          *progressbar.ProgressBar
	)
	if showProgress {
		// Use a spinner instead, since we do not know how many mods
		// there are, ahead of time.
		bar = progressbar.NewOptions(-1,
			progressbar.OptionShowCount(),
			progressbar.OptionSetPredictTime(false),
			progressbar.OptionSetDescription("Update cache"),
			progressbar.OptionSetWriter(os.Stderr),
		)
		defer bar.Exit()
	}

	var pullRequired bool
	c.withLock(func() error {
		pullRequired = c.cachedResultsPath == ""
		return nil
	})
	if pullRequired {
		if err := c.Pull(ctx); err != nil {
			return fmt.Errorf("pull mod list: %w", err)
		}
	}

	var resultsFile string
	c.withLock(func() error {
		resultsFile = c.cachedResultsPath
		return nil
	})
	f, err := os.Open(resultsFile)
	if err != nil {
		return fmt.Errorf("open results file: %s: %w", resultsFile, err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	return c.withTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		// Prepare statements.
		insertCategory, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO categories (name) VALUES (?)`)
		if err != nil {
			return fmt.Errorf("prepare insert category statement: %w", err)
		}

		insertMod, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO mods (name, title, owner, summary, category) VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("prepare insert mod statement: %w", err)
		}

		insertRelease, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO latest_releases (name, download_url, file_name, info_json, released_at, version, sha1) VALUES (?, ?, ?, json(?), ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("prepare insert release statement: %w", err)
		}

		for {
			var m modlistResult
			if err := dec.Decode(&m); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return fmt.Errorf("decode json: %w", err)
			}

			if _, err := insertCategory.ExecContext(ctx, m.Category); err != nil {
				return fmt.Errorf("insert into categories: %w", err)
			}

			if _, err := insertMod.ExecContext(ctx,
				m.Name,
				m.Title,
				m.Owner,
				m.Summary,
				m.Category,
			); err != nil {
				return fmt.Errorf("insert into mods: %w", err)
			}

			r := m.LatestRelease
			if _, err := insertRelease.ExecContext(ctx,
				m.Name,
				r.DownloadURL,
				r.FileName,
				r.InfoJSON,
				r.ReleasedAt.Format(time.RFC3339),
				r.Version,
				r.SHA1,
			); err != nil {
				return fmt.Errorf("insert into latest releases: %w", err)
			}

			bar.Add(1)
		}
		return nil
	})

}

// withTx wraps a function in a database transaction.
// Callers should not explicitly call [database/sql.Tx.Commit] or
// [database/sql.Tx.Rollback] in fn.
// If fn returns a non-nil error, withTx will roll back the transaction.
func (c *Cache) withTx(ctx context.Context, fn func(context.Context, *sql.Tx) error) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		if err := tx.Rollback(); err != nil {
			return fmt.Errorf("rollback: %w", err)
		}
		return err
	}

	return tx.Commit()
}

// Clean removes all temporary mod list pulls from the cache directory.
func (c *Cache) Clean() error {
	return c.withLock(func() error {
		pattern := filepath.Join(c.dir, "facmod-*", "results.json")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("glob %q: %w", pattern, err)
		}

		for _, m := range matches {
			dir := filepath.Dir(m)
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("recursively delete directory %q: %w", dir, err)
			}
		}

		return nil
	})
}

// Search returns a list of mods matching the search term, with zero or more of
// the given options applied.
//
// Search will return a non-nil error if the search term is an empty string,
// or if there is an error with any of the provided options.
func (c *Cache) Search(ctx context.Context, searchTerm string, options ...SearchOption) ([]M, error) {
	if searchTerm == "" {
		return nil, errors.New("empty search term")
	}

	sopts := searchOptions{term: searchTerm}
	for _, opt := range options {
		if err := opt(&sopts); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	// Build the query.
	//
	// SELECT m.name, m.summary, r.released_at, r.version
	// FROM mods AS m
	// JOIN latest_releases USING (name)
	// WHERE r.info_json ->> '$.factorio_version' >= '1.1'
	// AND m.name LIKE '%$1%'
	selectQuery := squirrel.Select(
		"m.name",
		"m.summary",
		"m.category",
		"r.released_at",
		"r.version",
	).
		From("mods AS m").
		Join("latest_releases AS r USING (name)").
		Where(squirrel.GtOrEq{`r.info_json ->> '$.factorio_version'`: "1.1"}).
		Where(squirrel.Like{"m.name": "%" + sopts.term + "%"})

	if sopts.sortByDate {
		selectQuery = selectQuery.OrderBy("r.released_at DESC")
	}

	if nc := len(sopts.categories); nc > 0 {
		cc := make([]string, nc)
		for i, c := range sopts.categories {
			cc[i] = string(c)
		}
		selectQuery = selectQuery.Where(squirrel.Eq{"m.category": cc})
	}

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	println("SQL: " + query)

	var mm []M
	if err := c.withLock(func() error {
		return c.withTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
			rows, err := tx.QueryContext(ctx, query, args...)
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				var name, summary, category, releasedAt, version string
				if err := rows.Scan(&name, &summary, &category, &releasedAt, &version); err != nil {
					return fmt.Errorf("scan row: %w", err)
				}

				relAt, err := time.Parse(time.RFC3339, releasedAt)
				if err != nil {
					return fmt.Errorf("parse released at timestamp: %w", err)
				}

				mm = append(mm, M{
					Name:       name,
					Versions:   []Version{parseVersion(version)},
					ReleasedAt: relAt,
					Summary:    summary,
					Category:   category,
				})
			}

			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("query database: %w", err)
	}

	return mm, err
}

// SearchOption is a functional option that can be passed to [Cache.Search] to
// adjust how searching is handled.
type SearchOption func(*searchOptions) error

type searchOptions struct {
	term string // The search term.

	// Options that apply to how term is used or interpreted.
	nameOnly bool // Only attempt to match the search term to a mod's name.
	isRegexp bool // Interpret term as a regular expression.

	// Options that filter the results.
	categories []Category // Limit the search term to these mod categories.

	// Options that pertain to filtering.
	sortByDate bool // Sort by released_at date, descending.
}

// NameOnly restricts the mod search to only match on a mod's name.
// By default, a mod's name and summary will be considered.
func NameOnly() SearchOption {
	return func(o *searchOptions) error {
		o.nameOnly = true
		return nil
	}
}

// RegexpTerm tells [Cache.Search] to treat the search term as a regular expression.
// When this option is provided, the search term will be compiled by [regexp.Compile]
// to ensure it is valid.
func RegexpTerm() SearchOption {
	return func(o *searchOptions) error {
		if _, err := regexp.Compile(o.term); err != nil {
			return fmt.Errorf("compile regexp: %w", err)
		}
		o.isRegexp = true
		return nil
	}
}

// WithCategories limits the results of a search to only return mods with the
// specified categories.
func WithCategories(categories ...Category) SearchOption {
	return func(o *searchOptions) error {
		if len(categories) == 0 {
			return nil
		}

		cc := make([]Category, len(categories))
		for i, c := range categories {
			switch c {
			case NoCategory, Content, Overhaul, Tweaks, Utilities,
				Scenarios, ModPacks, Localizations, Internal:
				cc[i] = c
			default:
				if string(c) == "" {
					continue
				}
				return fmt.Errorf("unknown category: %s", c)
			}
		}
		o.categories = cc

		return nil
	}
}

// SortByDate sorts the results by the date the latest version of the mod was
// released, in descending order (most-recently-released mod first).
func SortByDate() SearchOption {
	return func(o *searchOptions) error {
		o.sortByDate = true
		return nil
	}
}

// Category is used to describe a mod.
// Mods can only belong to a single category.
type Category string

const (
	NoCategory    Category = "no-category"
	Content                = "content"       // Mods introducing new content into the game.
	Overhaul               = "overhaul"      // Large total conversion mods.
	Tweaks                 = "tweaks"        // Small changes concerning balance, gameplay, or graphics.
	Utilities              = "utilities"     // Providing the player with new tools or adjusting the game interface, without fundamentally changing gameplay.
	Scenarios              = "scenarios"     // Scenarios, maps, puzzles.
	ModPacks               = "mod-packs"     // Collections of mods with tweaks to make them work together.
	Localizations          = "localizations" // Translations for other mods.
	Internal               = "internal"      // Lua libraries for use by other mods and submods that are parts of a larger mod.
)

// Categories returns a list of all available categories.
func Categories() []string {
	return []string{
		"",
		string(NoCategory),
		string(Content),
		string(Overhaul),
		string(Tweaks),
		string(Utilities),
		string(Scenarios),
		string(ModPacks),
		string(Localizations),
		string(Internal),
	}
}
