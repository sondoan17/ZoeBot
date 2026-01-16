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
