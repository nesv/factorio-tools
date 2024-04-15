package mods

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	semver "github.com/Masterminds/semver/v3"
)

// Info holds the data loaded from a mod's info.json file.
type Info struct {
	Name            string   `json:"name"`
	RawVersion      string   `json:"version"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Author          string   `json:"author"`
	Contact         string   `json:"contact"`
	Homepage        string   `json:"homepage"`
	FactorioVersion string   `json:"factorio_version"`
	RawDependencies []string `json:"dependencies"`
}

// LoadInfo loads mod information from the "info.json" file from a ZIP archive
// containing a mod.
func LoadInfo(zipPath string) (Info, error) {
	f, err := os.Open(zipPath)
	if err != nil {
		return Info{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	fileInfo, err := f.Stat()
	if err != nil {
		return Info{}, fmt.Errorf("stat: %w", err)
	}

	zr, err := zip.NewReader(f, fileInfo.Size())
	if err != nil {
		return Info{}, fmt.Errorf("new zip reader: %w", err)
	}

	var infojsonPath string
	for _, f := range zr.File {
		if filepath.Base(f.Name) == "info.json" {
			infojsonPath = f.Name
			break
		}
	}
	if infojsonPath == "" {
		return Info{}, errors.New("no info.json file found")
	}

	infojson, err := zr.Open(infojsonPath)
	if err != nil {
		return Info{}, fmt.Errorf("open %q: %w", infojsonPath, err)
	}
	defer infojson.Close()

	var info Info
	if err := json.NewDecoder(infojson).Decode(&info); err != nil {
		return Info{}, fmt.Errorf("decode json: %w", err)
	}

	return info, nil
}

func (i Info) Version() (*semver.Version, error) {
	return semver.NewVersion(i.RawVersion)
}

func (i Info) Dependencies() (Dependencies, error) {
	var (
		required  []Dependency
		optional  []Dependency
		conflicts []Dependency
	)
	for _, dep := range i.RawDependencies {
		d, err := ParseDependency(dep)
		if err != nil {
			return Dependencies{}, fmt.Errorf("parse dependency %q: %w", dep, err)
		}

		if d.Mode&ModeOptional == ModeOptional {
			optional = append(optional, d)
		} else if d.Mode&ModeConflict == ModeConflict {
			conflicts = append(conflicts, d)
		} else {
			required = append(required, d)
		}
	}

	return Dependencies{
		Required:  required,
		Optional:  optional,
		Conflicts: conflicts,
	}, nil
}

type Dependencies struct {
	Required  []Dependency
	Optional  []Dependency
	Conflicts []Dependency
}

type Dependency struct {
	Name    string
	Version *DependencyVersion
	Mode    DependencyMode
}

func ParseDependency(s string) (Dependency, error) {
	mode := ModeRequired
	for prefix, m := range map[string]DependencyMode{
		optionalPrefix:       ModeOptional,
		hiddenOptionalPrefix: ModeOptional | ModeHidden,
		conflictPrefix:       ModeConflict,
		dnaloPrefix:          ModeNoAffectLoadOrder,
	} {
		if strings.HasPrefix(s, prefix) {
			mode = m
			break
		}
	}

	// Is there an equality operator?
	op, opIndex := findEqOp(s)
	if opIndex == -1 {
		// There is no equality operator.
		// Everything after the mode sigil is the mod name.
		return Dependency{
			Name: strings.TrimSpace(strings.TrimPrefix(s, mode.String())),
			Mode: mode,
		}, nil
	}

	// Hey hey! There is an equality operator! Everything between the mode
	// sigil and the equality operator is the mod name.
	name := strings.TrimSpace(s[len(mode.String()):opIndex])

	vstr := strings.TrimSpace(s[opIndex+len(op):])
	version, err := semver.NewVersion(vstr)
	if err != nil {
		return Dependency{}, fmt.Errorf("parse version %q: %w", vstr, err)
	}

	return Dependency{
		Name: name,
		Mode: mode,
		Version: &DependencyVersion{
			Op:      op,
			Version: version,
		},
	}, nil
}

func (d Dependency) String() string {
	var b strings.Builder
	if d.Mode != ModeRequired {
		b.WriteString(d.Mode.String() + " ")
	}
	b.WriteString(d.Name)
	if d.Version != nil {
		b.WriteString(" " + d.Version.String())
	}
	return b.String()
}

// findEqOp returns the equality operator, and its index, from a dependency
// string.
// The equality operator is used to determine how a dependency's version should
// be interpreted.
// The equality operator is one of:
//
// * "<" (less-than)
// * "<=" (less-than-or-equal-to)
// * "=" (equal-to)
// * ">=" (greater-than-or-equal-to)
// * ">" (greater-than)
//
// findEqOp returns "", -1 if there is no equality operator in s.
func findEqOp(s string) (string, int) {
	for _, op := range []string{
		"<=",
		"<",
		">=",
		">",
		"=",
	} {
		i := strings.Index(s, op)
		if i != -1 {
			return op, i
		}
	}
	return "", -1
}

type DependencyVersion struct {
	// Op is the equality operator indicating how Version should be
	// interpreted.
	Op string

	// Version of the dependency.
	Version *semver.Version
}

func (d DependencyVersion) String() string {
	if d.Version == nil {
		return ""
	}
	return d.Version.String()
}

// Mode indicates a mod dependency's "mode".
type DependencyMode uint

const (
	ModeRequired          DependencyMode = 1 << iota // Required dependency.
	ModeOptional                                     // Optional dependency.
	ModeHidden                                       // Hidden, optional dependency.
	ModeConflict                                     // This dependency conflicts with the mod.
	ModeNoAffectLoadOrder                            // Dependency does not affect load order.
)

func (d DependencyMode) String() string {
	switch d {
	case ModeOptional:
		return "?"
	case ModeHidden | ModeOptional:
		return "(?)"
	case ModeConflict:
		return "!"
	case ModeNoAffectLoadOrder:
		return "~"
	}
	return ""
}

var (
	optionalPrefix       = "?"
	hiddenOptionalPrefix = "(?)"
	conflictPrefix       = "!"
	dnaloPrefix          = "~" // dnalo = does not affect load order
)
