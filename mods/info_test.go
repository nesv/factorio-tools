package mods

import (
	"testing"

	semver "github.com/Masterminds/semver/v3"
)

func TestParseDependency(t *testing.T) {
	tests := []struct {
		name string
		want Dependency
		fail bool
	}{
		{
			name: "! Explosive Excavation",
			want: Dependency{
				Name: "Explosive Excavation",
				Mode: ModeConflict,
			},
		},
		{
			name: "flib >= 0.12.0",
			want: Dependency{
				Name: "flib",
				Mode: ModeRequired,
				Version: &DependencyVersion{
					Op:      ">=",
					Version: semver.MustParse("0.12.0"),
				},
			},
		},
		{
			name: "(?) ElectricTrain",
			want: Dependency{
				Name: "ElectricTrain",
				Mode: ModeOptional | ModeHidden,
			},
		},
		{
			name: "(?) Flow Control >= 3.0.5",
			want: Dependency{
				Name: "Flow Control",
				Mode: ModeOptional | ModeHidden,
				Version: &DependencyVersion{
					Op:      ">=",
					Version: semver.MustParse("3.0.5"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDependency(tt.name)
			if err != nil && !tt.fail {
				t.Fatal(err)
			} else if err == nil && tt.fail {
				t.Fatal("should have failed")
			}

			if tt.want.Name != got.Name {
				t.Errorf("name: want=%q got=%q", tt.want.Name, got.Name)
			}

			if tt.want.Mode != got.Mode {
				t.Errorf("mode: want=%q got=%q", tt.want.Mode, got.Mode)
			}

			if tt.want.Version != nil {
				if got.Version == nil {
					t.Fatal("version: wanted non-nil verison, got nil version")
				}

				if tt.want.Version.Op != got.Version.Op {
					t.Errorf("version op: want=%q got=%q", tt.want.Version.Op, got.Version.Op)
				}

				if want, got := tt.want.Version.Version, got.Version.Version; !want.Equal(got) {
					t.Errorf("version: want=%q got=%q", want, got)
				}
			} else if tt.want.Version == nil && got.Version != nil {
				t.Error("version: wanted nil version, got non-nil version")
			}
		})
	}
}
