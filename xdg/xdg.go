// Package xdg provides convenience functions for building paths compliant with
// the [XDG Base Directory Specification].
//
// This package only contains functions that are not otherwise provided by the
// Go standard library.
// If you wish to retrieve the user-specific cache or configuration directories,
// see [os.UserCacheDir] and [os.UserConfigDir] respectively.
//
// [XDG Base Directory Specification]: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
package xdg

import (
	"errors"
	"os"
	"path/filepath"
)

// UserStateDir returns the default root directory to use for user-specific
// state files. Users should create their own application-specific subdirectory
// within this one and use that.
//
// If the location cannot be determined (for example, $HOME is not defined),
// then a non-nil error will be returned.
func UserStateDir() (string, error) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = os.Getenv("HOME")
		if dir == "" {
			return "", errors.New("neither $XDG_STATE_HOME nor $HOME are defined")
		}
		dir = filepath.Join(dir, ".local", "state")
	}
	return dir, nil
}
