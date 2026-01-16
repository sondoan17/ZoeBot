// Package data provides game data loaders for ZoeBot.
package data

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// ItemEntry represents a single item from the JSON array
type ItemEntry struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	IconPath string `json:"iconPath"`
}

// PerkData represents a single perk/rune
type PerkData struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	IconPath string `json:"iconPath"`
}

// PerkStyleData represents the structure of perk-style.json
type PerkStyleData struct {
	SchemaVersion int         `json:"schemaVersion"`
	Styles        []PerkStyle `json:"styles"`
}

// PerkStyle represents a single rune tree (e.g., Precision, Domination)
type PerkStyle struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var (
	itemData     map[int]ItemEntry // id -> item
	itemDataOnce sync.Once
	itemDataErr  error

	perkData     map[int]PerkData
	perkDataOnce sync.Once
	perkDataErr  error

	perkStyleData     map[int]string // id -> name
	perkStyleDataOnce sync.Once
	perkStyleDataErr  error

	championData     map[string]ChampionEntry // alias (lowercase) -> champion
	championDataOnce sync.Once
	championDataErr  error
)

// ChampionEntry represents a champion from champion-summary.json
type ChampionEntry struct {
	ID                 int      `json:"id"`
	Name               string   `json:"name"`
	Alias              string   `json:"alias"`
	SquarePortraitPath string   `json:"squarePortraitPath"`
	Roles              []string `json:"roles"`
}

// LoadItems loads item data from the JSON file (array format)
func LoadItems(filePath string) (map[int]ItemEntry, error) {
	itemDataOnce.Do(func() {
		data, err := os.ReadFile(filePath)
		if err != nil {
			itemDataErr = err
			return
		}

		var items []ItemEntry
		if err := json.Unmarshal(data, &items); err != nil {
			itemDataErr = err
			return
		}

		itemData = make(map[int]ItemEntry)
		for _, item := range items {
			itemData[item.ID] = item
		}
	})

	return itemData, itemDataErr
}

// LoadPerks loads perk/rune data from the JSON file
func LoadPerks(filePath string) (map[int]PerkData, error) {
	perkDataOnce.Do(func() {
		data, err := os.ReadFile(filePath)
		if err != nil {
			perkDataErr = err
			return
		}

		var perks []PerkData
		if err := json.Unmarshal(data, &perks); err != nil {
			perkDataErr = err
			return
		}

		perkData = make(map[int]PerkData)
		for _, p := range perks {
			perkData[p.ID] = p
		}
	})

	return perkData, perkDataErr
}

// GetItemName returns the Vietnamese name for an item ID
func GetItemName(itemID string) string {
	if itemData == nil {
		return "Item #" + itemID
	}

	// Convert string ID to int
	id, err := strconv.Atoi(itemID)
	if err != nil {
		return "Item #" + itemID
	}

	if item, ok := itemData[id]; ok {
		return item.Name
	}

	return "Item #" + itemID
}

// GetPerkName returns the Vietnamese name for a perk/rune ID
func GetPerkName(perkID int) string {
	if perkData == nil {
		return ""
	}

	if perk, ok := perkData[perkID]; ok {
		return perk.Name
	}

	return ""
}

// GetItems returns all loaded items
func GetItems() map[int]ItemEntry {
	return itemData
}

// GetPerks returns all loaded perks
func GetPerks() map[int]PerkData {
	return perkData
}

// LoadPerkStyles loads perk style (rune tree) data from the JSON file
func LoadPerkStyles(filePath string) (map[int]string, error) {
	perkStyleDataOnce.Do(func() {
		data, err := os.ReadFile(filePath)
		if err != nil {
			perkStyleDataErr = err
			return
		}

		var styleData PerkStyleData
		if err := json.Unmarshal(data, &styleData); err != nil {
			perkStyleDataErr = err
			return
		}

		perkStyleData = make(map[int]string)
		for _, style := range styleData.Styles {
			perkStyleData[style.ID] = style.Name
		}
	})

	return perkStyleData, perkStyleDataErr
}

// GetPerkStyleName returns the Vietnamese name for a perk style/tree ID
func GetPerkStyleName(styleID int) string {
	if perkStyleData == nil {
		return ""
	}

	if name, ok := perkStyleData[styleID]; ok {
		return name
	}

	return ""
}

// GetPerkIconURL returns the CDN URL for a perk icon
func GetPerkIconURL(perkID int) string {
	if perkData == nil {
		return ""
	}

	if perk, ok := perkData[perkID]; ok && perk.IconPath != "" {
		// Convert path like "/lol-game-data/assets/v1/perk-images/Styles/..."
		// to Community Dragon URL
		iconPath := perk.IconPath
		// Remove the prefix and convert to lowercase for CDN
		iconPath = strings.TrimPrefix(iconPath, "/lol-game-data/assets/v1/")
		return "https://raw.communitydragon.org/latest/plugins/rcp-be-lol-game-data/global/default/v1/" + strings.ToLower(iconPath)
	}

	return ""
}

// GetItemIconURL returns the Data Dragon URL for an item icon
func GetItemIconURL(itemID string, version string) string {
	if version == "" {
		version = "14.24.1" // fallback version
	}
	return fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/img/item/%s.png", version, itemID)
}

// LoadChampions loads champion data from the JSON file
func LoadChampions(filePath string) (map[string]ChampionEntry, error) {
	championDataOnce.Do(func() {
		data, err := os.ReadFile(filePath)
		if err != nil {
			championDataErr = err
			return
		}

		var champions []ChampionEntry
		if err := json.Unmarshal(data, &champions); err != nil {
			championDataErr = err
			return
		}

		championData = make(map[string]ChampionEntry)
		for _, champ := range champions {
			// Store by lowercase alias for easy lookup
			key := strings.ToLower(champ.Alias)
			championData[key] = champ
			// Also store by lowercase name
			nameKey := strings.ToLower(champ.Name)
			if nameKey != key {
				championData[nameKey] = champ
			}
		}
	})

	return championData, championDataErr
}

// GetChampionIconURL returns the CDN URL for a champion icon
func GetChampionIconURL(championName string) string {
	if championData == nil {
		return ""
	}

	// Normalize name for lookup
	key := strings.ToLower(championName)
	key = strings.ReplaceAll(key, " ", "")
	key = strings.ReplaceAll(key, "'", "")

	if champ, ok := championData[key]; ok && champ.SquarePortraitPath != "" {
		// Convert path to Community Dragon URL
		iconPath := champ.SquarePortraitPath
		iconPath = strings.TrimPrefix(iconPath, "/lol-game-data/assets/v1/")
		return "https://raw.communitydragon.org/latest/plugins/rcp-be-lol-game-data/global/default/v1/" + strings.ToLower(iconPath)
	}

	// Try original name
	if champ, ok := championData[strings.ToLower(championName)]; ok && champ.SquarePortraitPath != "" {
		iconPath := champ.SquarePortraitPath
		iconPath = strings.TrimPrefix(iconPath, "/lol-game-data/assets/v1/")
		return "https://raw.communitydragon.org/latest/plugins/rcp-be-lol-game-data/global/default/v1/" + strings.ToLower(iconPath)
	}

	return ""
}
