package userdata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PlayerData holds the information from the game's "player-data.json" file.
//
// NOTE: Currently, only ServiceToken and ServiceUsername are needed.
type PlayerData struct {
	ServiceToken    string `json:"service-token"`
	ServiceUsername string `json:"service-username"`

	// These fields are currently unused by the rest of this module.
	AvailableCampaignLevels      json.RawMessage `json:"available-campaign-levels"`
	BlueprintView                json.RawMessage `json:"blueprint-view"`
	ConsoleHistory               json.RawMessage `json:"console-history"`
	EditorLuaSnippets            json.RawMessage `json:"editor-lua-snippets"`
	LastPlayed                   json.RawMessage `json:"last-played"`
	LastPlayedVersion            json.RawMessage `json:"last-played-version"`
	LatestMultiplayerConnections json.RawMessage `json:"latest-multiplayer-connections"`
	MainMenuSimulationsPlayer    json.RawMessage `json:"main-menu-simulations-player"`
	ShortcutBar                  json.RawMessage `json:"shortcut-bar"`
	Tips                         json.RawMessage `json:"tips"`
}

func LoadPlayerData(installDir string) (PlayerData, error) {
	name := filepath.Join(installDir, "player-data.json")
	f, err := os.Open(name)
	if err != nil {
		return PlayerData{}, fmt.Errorf("open player-data.json: %w", err)
	}
	defer f.Close()

	var pdata PlayerData
	if err := json.NewDecoder(f).Decode(&pdata); err != nil {
		return PlayerData{}, fmt.Errorf("decode json: %w", err)
	}

	return pdata, nil
}
