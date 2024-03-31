// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package server

import (
	"encoding/json"
	"fmt"
	"io"
)

// DefaultSettings returns [Settings] with default values set.
func DefaultSettings() *Settings {
	return &Settings{
		Tags:                        []string{},
		MaxUploadSlots:              5,
		MaxHeartbeatsPerSecond:      60,
		AllowCommands:               "admins-only",
		AutosaveInterval:            10,
		AutosaveSlots:               5,
		AutoPause:                   true,
		OnlyAdminsCanPauseTheGame:   true,
		AutosaveOnlyOnServer:        true,
		MinimumSegmentSize:          25,
		MinimumSegmentSizePeerCount: 20,
		MaximumSegmentSize:          100,
		MaximumSegmentSizePeerCount: 10,
	}
}

// Settings holds the settings for the Factorio game server.
type Settings struct {
	// Name of the game as it will appear in the game listing.
	Name string `json:"name"`

	// Description of the game that will appear in the listing.
	Description string `json:"description"`

	// Game tags.
	Tags []string `json:"tags"`

	// Maximum number of players allowed.
	// Admins can join even a full server.
	// 0 means unlimited.
	MaxPlayers uint `json:"max_players"`

	// Server visibility in the game listing.
	Visibility Visibility `json:"visibility"`

	// Your factorio.com login credentials.
	// Required for games with visibility public.
	Username string `json:"username"`
	Password string `json:"password"`

	// Authentication token.
	// May be used instead of Password.
	Token string `json:"token"`

	// Optional password that users must provide if they wish to join your game.
	// An empty string means no password will be required.
	GamePassword string `json:"game_password"` // default: ""

	// When set to true, the server will only allow clients that have a valid factorio.com account.
	RequireUserVerification bool `json:"require_user_verification"` // default: false

	// Optional, default value is 0 (unlimited).
	MaxUploadInKilobytesPerSecond uint `json:"max_upload_in_kilobytes_per_second"` // default: 0

	// Optional, default value is 5.
	// 0 means unlimited.
	MaxUploadSlots uint `json:"max_upload_slots"` // default: 5

	// Optional.
	// One tick is 16ms in default speed.
	// 0 means no minimum.
	MinimumLatencyInTicks uint `json:"minimum_latency_in_ticks"` // default: 0

	// Network tick rate.
	// Maximum rate game updates packets are sent at before bundling them together.
	// Minimum value is 5, maximum value is 240.
	MaxHeartbeatsPerSecond uint `json:"max_heartbeats_per_second"` // default: 60

	// Players that played on this map already can join, even when the max player limit is reached.
	IgnorePlayerLimitForReturningPlayers bool `json:"ignore_player_limit_for_returning_players"` // default: false

	// Set who is allowed to issue commands through the in-game console.
	// Possible values are "true", "false", and "admins-only".
	AllowCommands string `json:"allow_commands"` // default: admins-only

	// Autosave interval, in minutes.
	AutosaveInterval uint `json:"autosave_interval"` // default: 10

	// Server autosave slots.
	// It is cycled through when the server autosaves.
	AutosaveSlots uint `json:"autosave_slots"` // default: 5

	// How many minutes until someone is kicked when doing nothing.
	// 0 for never.
	AFKAutokickInterval uint `json:"afk_autokick_interval"` // default: 0

	// Whether the server should be paused when no players are present.
	AutoPause bool `json:"auto_pause"` // default: true

	// Only allow admins to pause the game.
	OnlyAdminsCanPauseTheGame bool `json:"only_admins_can_pause_the_game"` // default: true

	// Whether autosaves should be saved only on the server, or also on all connected clients.
	AutosaveOnlyOnServer bool `json:"autosave_only_on_server"` // default: true

	// Highly experimental feature.
	// Enable only at your own risk of losing yoursaves.
	// On UNIX systems, the server will fork itself to create an autosave.
	// Autosaving on connected Windows clients will be disabled regardless of AutosaveOnlyOnServer.
	NonBlockingSaving bool `json:"non_blocking_saving"` // default: false

	// Long network messages are split into segments that are sent over multiple ticks.
	// Their size depends on the number of peers currently connected.
	// Increasing the segment size will increase upload bandwidth requirements for the server and download bandwidth requirements for clients.
	// This setting only affects server outbound messages.
	// Changing these setings can have a negative impact on connection stability for some clients.
	MinimumSegmentSize          uint `json:"minimum_segment_size"`            // default: 25
	MinimumSegmentSizePeerCount uint `json:"minimum_segment_size_peer_count"` // default: 20
	MaximumSegmentSize          uint `json:"maximum_segment_size"`            // default: 100
	MaximumSegmentSizePeerCount uint `json:"maximum_segment_size_peer_count"` // default: 10
}

// Visibility controls how the Factorio server will advertise itself.
type Visibility struct {
	// Game will be published onthe official Factorio matching server.
	// When this option is set to true, [Settings.Username] needs to be set,
	// along with one of [Settings.Password] or [Settings.Token].
	Public bool `json:"public"` // default: false

	// Game will be broadcast on LAN (local area network).
	LAN bool `json:"lan"` // default: false
}

// ReadFrom implements the [io.ReaderFrom] interface, populating the values in s from the contents in r.
// On a successful invocation, ReadFrom will return 0, nil.
func (s *Settings) ReadFrom(r io.Reader) (int64, error) {
	dec := json.NewDecoder(r)
	if err := dec.Decode(s); err != nil {
		return 0, fmt.Errorf("decode json: %w", err)
	}
	return 0, nil
}

// WriteTo implements the [io.WriterTo] interface, and will encode the data in s to w.
// On a successful invocation, WriteTo returns 0, nil.
func (s *Settings) WriteTo(w io.Writer) (int64, error) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return 0, fmt.Errorf("encode json: %w", err)
	}
	return 0, nil
}
