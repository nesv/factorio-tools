// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mods

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Load collects all of the mods currently installed to the installation directory.
func Load(installationDir string) ([]M, error) {
	modDir := filepath.Join(installationDir, "mods")
	f, err := os.Open(filepath.Join(modDir, "mod-list.json"))
	if err != nil {
		return nil, fmt.Errorf("open mod list: %w", err)
	}
	defer f.Close()

	var list modlistjson
	if err := json.NewDecoder(f).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	mods := make([]M, len(list.Mods))
	for i, m := range list.Mods {
		if err := m.findInstalledVersions(installationDir); err != nil {
			return nil, fmt.Errorf("find installed versions: %w", err)
		}
		mods[i] = m
	}
	slices.SortFunc(mods, func(a, b M) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name == b.Name {
			return 0
		}
		return 1
	})

	return mods, nil
}

type modlistjson struct {
	Mods []M `json:"mods"`
}

type M struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`

	// The following fields are not a part of the mod-list.json file.

	// All of the currently-installed versions of the mod, sorted in
	// ascending order so the latest version is the last element in the
	// slice.
	Versions []Version `json:"-"`

	// The time at which the latest version was released.
	ReleasedAt time.Time `json:"-"`

	// A brief summary of the mod.
	Summary string `json:"-"`

	// The mod's category.
	Category string `json:"-"`
}

func (m *M) findInstalledVersions(installDir string) error {
	pattern := filepath.Join(installDir, "mods", fmt.Sprintf("%s_*.zip", m.Name))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	versions := make([]Version, len(matches))
	for i, match := range matches {
		mp := modpath(match)
		versions[i] = mp.version()
	}
	slices.SortFunc(versions, func(a, b Version) int {
		if a.Major > b.Major {
			return 3
		} else if a.Major < b.Major {
			return -3
		}
		if a.Minor > b.Minor {
			return 2
		} else if a.Minor < b.Minor {
			return -2
		}
		if a.Patch > b.Patch {
			return 1
		} else if a.Patch < b.Patch {
			return -1
		}
		return 0
	})
	m.Versions = versions

	return nil
}

type modpath string

func (m modpath) version() Version {
	base := filepath.Base(string(m))
	i := strings.LastIndex(base, "_")
	if i == -1 {
		return Version{}
	}
	vs := base[i+1 : strings.LastIndex(base, ".zip")]
	return parseVersion(vs)
}

func parseVersion(version string) Version {
	fields := strings.SplitN(version, ".", 3)
	var major, minor, patch int
	if len(fields) >= 1 {
		n, err := strconv.Atoi(fields[0])
		if err == nil {
			major = n
		}
	}
	if len(fields) >= 2 {
		n, err := strconv.Atoi(fields[1])
		if err == nil {
			minor = n
		}
	}
	if len(fields) == 3 {
		n, err := strconv.Atoi(fields[2])
		if err == nil {
			patch = n
		}
	}
	return Version{Major: major, Minor: minor, Patch: patch}
}

type Version struct {
	Major, Minor, Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) IsZero() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0
}
