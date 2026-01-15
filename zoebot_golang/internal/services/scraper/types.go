package scraper

// CounterStats represents data for a single counter matchup.
type CounterStats struct {
	ChampionName string `json:"champion_name"`
	WinRate      string `json:"win_rate"` // e.g., "54.5%"
	Matches      string `json:"matches"`  // e.g., "1,200"
	Lane         string `json:"lane"`
}

// Scraper defines the interface for fetching counter data.
type Scraper interface {
	GetCounters(champion, lane string) ([]*CounterStats, error)
}
