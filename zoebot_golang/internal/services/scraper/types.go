package scraper

// CounterStats represents data for a single counter matchup.
type CounterStats struct {
	ChampionName string `json:"champion_name"`
	WinRate      string `json:"win_rate"` // e.g., "54.5%"
	Lane         string `json:"lane"`
}

// CounterData contains both best and worst picks.
type CounterData struct {
	BestPicks  []*CounterStats `json:"best_picks"`  // Champions that counter the target
	WorstPicks []*CounterStats `json:"worst_picks"` // Champions weak against the target
	Lane       string          `json:"lane"`
}

// Scraper defines the interface for fetching counter data.
type Scraper interface {
	GetCounters(champion, lane string) (*CounterData, error)
}

// BuildData represents champion build information from OP.GG.
type BuildData struct {
	Champion     string `json:"champion"`
	Role         string `json:"role"`
	PatchVersion string `json:"patch_version"`

	// Runes
	PrimaryTree     string   `json:"primary_tree"`      // Precision, Domination, etc.
	PrimaryRunes    []string `json:"primary_runes"`     // 4 runes (keystone + 3)
	SecondaryTree   string   `json:"secondary_tree"`    // Secondary tree name
	SecondaryRunes  []string `json:"secondary_runes"`   // 2 secondary runes
	StatShards      []string `json:"stat_shards"`       // 3 stat shards
	RuneWinRate     string   `json:"rune_win_rate"`     // e.g., "54.5%"
	RunePickRate    string   `json:"rune_pick_rate"`    // e.g., "32.1%"
	RuneGamesPlayed string   `json:"rune_games_played"` // e.g., "12,345"

	// Items
	StarterItems    []string `json:"starter_items"`     // Starting items
	Boots           string   `json:"boots"`             // Boots name
	CoreItems       []string `json:"core_items"`        // 3 core items
	ItemWinRate     string   `json:"item_win_rate"`     // e.g., "55.2%"
	ItemPickRate    string   `json:"item_pick_rate"`    // e.g., "45.3%"
	ItemGamesPlayed string   `json:"item_games_played"` // e.g., "8,765"
}

// RuneTreeNames maps rune tree IDs to English names.
var RuneTreeNames = map[int]string{
	8000: "Precision",
	8100: "Domination",
	8200: "Sorcery",
	8300: "Inspiration",
	8400: "Resolve",
}

// RuneNames maps rune IDs to English names.
var RuneNames = map[int]string{
	// Precision Keystones
	8005: "Press the Attack",
	8008: "Lethal Tempo",
	8021: "Fleet Footwork",
	8010: "Conqueror",
	// Precision Minor
	9101: "Absorb Life",
	9111: "Triumph",
	8009: "Presence of Mind",
	9104: "Legend: Alacrity",
	9105: "Legend: Haste",
	9103: "Legend: Bloodline",
	8014: "Coup de Grace",
	8017: "Cut Down",
	8299: "Last Stand",

	// Domination Keystones
	8112: "Electrocute",
	8124: "Predator",
	8128: "Dark Harvest",
	9923: "Hail of Blades",
	// Domination Minor
	8126: "Cheap Shot",
	8139: "Taste of Blood",
	8143: "Sudden Impact",
	8136: "Zombie Ward",
	8120: "Ghost Poro",
	8138: "Eyeball Collection",
	8135: "Treasure Hunter",
	8134: "Ingenious Hunter",
	8105: "Relentless Hunter",
	8106: "Ultimate Hunter",

	// Sorcery Keystones
	8214: "Summon Aery",
	8229: "Arcane Comet",
	8230: "Phase Rush",
	// Sorcery Minor
	8224: "Nullifying Orb",
	8226: "Manaflow Band",
	8275: "Nimbus Cloak",
	8210: "Transcendence",
	8234: "Celerity",
	8233: "Absolute Focus",
	8237: "Scorch",
	8232: "Waterwalking",
	8236: "Gathering Storm",

	// Resolve Keystones
	8437: "Grasp of the Undying",
	8439: "Aftershock",
	8465: "Guardian",
	// Resolve Minor
	8446: "Demolish",
	8463: "Font of Life",
	8401: "Shield Bash",
	8429: "Conditioning",
	8444: "Second Wind",
	8473: "Bone Plating",
	8451: "Overgrowth",
	8453: "Revitalize",
	8242: "Unflinching",

	// Inspiration Keystones
	8351: "Glacial Augment",
	8360: "Unsealed Spellbook",
	8369: "First Strike",
	// Inspiration Minor
	8306: "Hextech Flashtraption",
	8304: "Magical Footwear",
	8313: "Triple Tonic",
	8321: "Future's Market",
	8316: "Minion Dematerializer",
	8345: "Biscuit Delivery",
	8347: "Cosmic Insight",
	8410: "Approach Velocity",
	8352: "Time Warp Tonic",
	8358: "Jack of All Trades",
}

// StatShardNames maps stat shard IDs to names.
var StatShardNames = map[int]string{
	5001: "Health Scaling",
	5002: "Armor",
	5003: "Magic Resist",
	5005: "Attack Speed",
	5007: "Ability Haste",
	5008: "Adaptive Force",
	5010: "Move Speed",
	5011: "Health",
	5013: "Tenacity",
}
