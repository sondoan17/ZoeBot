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
	KeystoneID      int      `json:"keystone_id"`       // Keystone rune ID for icon

	// Items
	StarterItems    []string `json:"starter_items"`     // Starting items
	Boots           string   `json:"boots"`             // Boots name
	CoreItems       []string `json:"core_items"`        // 3 core items
	ItemWinRate     string   `json:"item_win_rate"`     // e.g., "55.2%"
	ItemPickRate    string   `json:"item_pick_rate"`    // e.g., "45.3%"
	ItemGamesPlayed string   `json:"item_games_played"` // e.g., "8,765"
}
