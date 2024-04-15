package server

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/nesv/factorio-tools/mods"
)

type Installation struct {
	Settings Settings

	dir string
}

// Open collects information about a Factorio headless server installation from
// the provided installation directory.
//
// If the directory does not exist, Open will return [io/fs.ErrNoExist].
// If the path referred to by dir exists but is not a directory, or not a
// directory containing a Factorio server installation, Open will return a
// non-nil error.
//
// To create a new installation, see [Install].
func Open(dir string) (*Installation, error) {
	if exists, err := dirExists(dir); err != nil {
		return nil, err
	} else if !exists {
		return nil, fs.ErrNotExist
	}

	settings, err := LoadSettings(dir)
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	return &Installation{
		Settings: settings,
		dir:      dir,
	}, nil
}

func Install(dir string) (*Installation, error) { return nil, errors.New("not implemented") }

func (i *Installation) Mods() ([]mods.M, error) {
	pattern := filepath.Join(i.dir, "mods", "*.zip")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	// Find all mods, by name.
	mm := map[string]struct{}{}
	for _, match := range matches {
		nameVersion := strings.TrimSuffix(filepath.Base(match), ".zip")
		name, _, found := strings.Cut(nameVersion, "_")
		if !found {
			return nil, fmt.Errorf("missing _ separator in mod file name: %s", filepath.Base(match))
		}
		mm[name] = struct{}{}
	}

	// Now that we have the names of all installed mods, resolve their versions.
	mms := make([]mods.M, len(mm))
	for name := range mm {
		m := mods.M{Name: name}
		if err := m.FindInstalledVersions(i.dir); err != nil {
			return nil, fmt.Errorf("find installed versions of %q: %v", name, err)
		}
	}

	return mms, nil
}
