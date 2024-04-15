package server

import (
	"errors"
	"fmt"
	"io/fs"

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
	return nil, errors.New("not implemented")
}
