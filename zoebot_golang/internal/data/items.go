// Package data provides game data loaders for ZoeBot.
package data

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// ItemData represents the structure of item.json
type ItemData struct {
	Type    string                `json:"type"`
	Version string                `json:"version"`
	Data    map[string]ItemDetail `json:"data"`
}

// ItemDetail represents a single item's data
type ItemDetail struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Plaintext   string `json:"plaintext"`
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
	itemData     *ItemData
	itemDataOnce sync.Once
	itemDataErr  error

	perkData     map[int]PerkData
	perkDataOnce sync.Once
	perkDataErr  error

	perkStyleData     map[int]string // id -> name
	perkStyleDataOnce sync.Once
	perkStyleDataErr  error
)

// LoadItems loads item data from the JSON file
func LoadItems(filePath string) (*ItemData, error) {
	itemDataOnce.Do(func() {
		data, err := os.ReadFile(filePath)
		if err != nil {
			itemDataErr = err
			return
		}

		itemData = &ItemData{}
		if err := json.Unmarshal(data, itemData); err != nil {
			itemDataErr = err
			itemData = nil
			return
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

	if item, ok := itemData.Data[itemID]; ok {
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
func GetItems() map[string]ItemDetail {
	if itemData == nil {
		return nil
	}
	return itemData.Data
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
