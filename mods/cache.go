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
	"sync"

	progressbar "github.com/schollz/progressbar/v3"
	_ "modernc.org/sqlite"

	"git.sr.ht/~nesv/factorio-tools/httputil"
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
				r.ReleasedAt,
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
