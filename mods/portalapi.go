// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mods

import (
	"encoding/json"
	"time"
)

type modlist struct {
	Pagination pagination      `json:"pagination"`
	Results    []modlistResult `json:"results"`
}

type pagination struct {
	Count     int             `json:"count"`      // Total number of mods that matched filters
	Links     paginationLinks `json:"links"`      // Links to mod portal api request, preserving all filters and search queries
	Page      int             `json:"page"`       // Current page number
	PageCount int             `json:"page_count"` // Total number of pages returned
	PageSize  int             `json:"page_size"`  // Number of results per page
}

type paginationLinks struct {
	First *string `json:"first"`
	Prev  *string `json:"prev"`
	Next  *string `json:"next"`
	Last  *string `json:"last"`
}

type modlistResult struct {
	// Available on all endpoints.
	DownloadsCount int          `json:"downloads_count"` // Number of downloads
	Name           string       `json:"name"`            // Machine-readable ID
	Owner          string       `json:"owner"`           // Factorio username of the mod's author
	Releases       []modRelease `json:"releases"`        // Available versions of the mod available for download
	Summary        string       `json:"summary"`         // Short mod description
	Title          string       `json:"title"`           // Human-readable name for the mod
	Category       string       `json:"category"`        // Single category describing the mod

	// Only available on the "/api/mods" endpoint.
	LatestRelease modRelease `json:"latest_release"` // Latest version of the mod available for download

	// Available on the "short" and "full" endpoints.
	Thumbnail string `json:"thumbnail"` // Relative URL path to the thumbnail of the mod

	// Available on the "full" endpoint.
	Changelog   string     `json:"changelog"`   // Recent changes to the mod
	CreatedAt   time.Time  `json:"created_at"`  // When the mod was created
	Description string     `json:"description"` // Longer description of the mod, in text-only format
	SourceURL   string     `json:"source_url"`  // URL to the mod's source code
	Homepage    string     `json:"homepage"`    // URL to the mod's main project page, but could be anything
	Tags        []string   `json:"tags"`        // List of tag names to categorize the mod
	License     modLicense `json:"license"`     // License that applies to the mod
}

func (r modlistResult) thumbnailURL() string {
	relpath := r.Thumbnail
	if relpath == "" {
		relpath = "/assets/.thumb.png"
	}
	return "https://assets-mod.factorio.com" + r.Thumbnail
}

type modRelease struct {
	DownloadURL string    `json:"download_url"`
	FileName    string    `json:"file_name"`
	ReleasedAt  time.Time `json:"released_at"`
	Version     string    `json:"version"`
	SHA1        string    `json:"sha1"`

	// Copy of the mod's info.json file.
	// In the "/api/mod/{name}" (a.k.a. "short") endpoint, only contains
	// "factorio_version".
	// In the "/api/mod/{name}/full" endpoint, also contains an array of
	// dependencies.
	InfoJSON json.RawMessage `json:"info_json"`
}

type modLicense struct {
	Description string `json:"description"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	URL         string `json:"url"`
}
