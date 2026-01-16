package scraper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zoebot/internal/storage"
)

// Client is the scraper client.
type Client struct {
	httpClient *http.Client
	redis      *storage.RedisClient
}

// NewClient creates a new scraper client.
func NewClient(redis *storage.RedisClient) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		redis:      redis,
	}
}

// U.GG GraphQL API endpoint
const uggAPIURL = "https://u.gg/api"

// GraphQL query for matchups
const matchupsQuery = `
query GetChampionMatchups($championId: Int!, $role: Role, $patch: String, $region: Region, $rank: Rank, $queueType: QueueType) {
  championMatchups(
    championId: $championId
    role: $role
    patch: $patch
    region: $region
    rank: $rank
    queueType: $queueType
  ) {
    counters {
      championId
      wins
      games
    }
    goodMatchups {
      championId
      wins
      games
    }
  }
}
`

// GraphQL request/response types
type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type matchupsResponse struct {
	Data struct {
		ChampionMatchups struct {
			Counters []struct {
				ChampionID int `json:"championId"`
				Wins       int `json:"wins"`
				Games      int `json:"games"`
			} `json:"counters"`
			GoodMatchups []struct {
				ChampionID int `json:"championId"`
				Wins       int `json:"wins"`
				Games      int `json:"games"`
			} `json:"goodMatchups"`
		} `json:"championMatchups"`
	} `json:"data"`
}

// GetCounters returns a list of counter champions.
func (c *Client) GetCounters(champion, lane string) ([]*CounterStats, error) {
	// Normalize inputs
	normChamp := normalizeChampionName(champion)
	normLane := normalizeLane(lane)

	// Redis Key: counter:v2:{champ}:{lane}
	cacheKey := fmt.Sprintf("counter:v2:%s:%s", normChamp, normLane)

	// 1. Check Cache
	if c.redis != nil {
		if val, err := c.redis.Get(cacheKey); err == nil && val != "" {
			var stats []*CounterStats
			if err := json.Unmarshal([]byte(val), &stats); err == nil {
				log.Printf("Counter cache hit for %s %s", champion, lane)
				return stats, nil
			}
		}
	}

	// 2. Fetch from U.GG API
	log.Printf("Fetching counters for %s %s from U.GG API", champion, lane)

	// Get champion ID from name
	champID := getChampionID(normChamp)
	if champID == 0 {
		return nil, fmt.Errorf("champion not found: %s", champion)
	}

	stats, err := c.fetchMatchupsAPI(champID, normLane)
	if err != nil {
		// Fallback to web scraping API
		stats, err = c.fetchCountersFromWebAPI(normChamp, normLane)
		if err != nil {
			return nil, err
		}
	}

	// 3. Save to Cache
	if c.redis != nil && len(stats) > 0 {
		data, _ := json.Marshal(stats)
		c.redis.Set(cacheKey, string(data))
	}

	return stats, nil
}

// fetchMatchupsAPI tries to fetch data from U.GG GraphQL API
func (c *Client) fetchMatchupsAPI(champID int, lane string) ([]*CounterStats, error) {
	variables := map[string]interface{}{
		"championId": champID,
		"queueType":  "RANKED_SOLO_5X5",
		"rank":       "PLATINUM_PLUS",
		"region":     "WORLD",
	}

	if lane != "" {
		variables["role"] = strings.ToUpper(lane)
	}

	reqBody := graphQLRequest{
		Query:     matchupsQuery,
		Variables: variables,
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", uggAPIURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var result matchupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	counters := result.Data.ChampionMatchups.Counters
	if len(counters) == 0 {
		return nil, fmt.Errorf("no counters data from API")
	}

	var stats []*CounterStats
	for i, counter := range counters {
		if i >= 10 {
			break
		}
		winRate := 0.0
		if counter.Games > 0 {
			winRate = float64(counter.Wins) / float64(counter.Games) * 100
		}
		stats = append(stats, &CounterStats{
			ChampionName: getChampionName(counter.ChampionID),
			WinRate:      fmt.Sprintf("%.2f%%", winRate),
			Matches:      fmt.Sprintf("%d", counter.Games),
			Lane:         lane,
		})
	}

	return stats, nil
}

// fetchCountersFromWebAPI uses U.GG's stats API endpoint as fallback
func (c *Client) fetchCountersFromWebAPI(champion, lane string) ([]*CounterStats, error) {
	// U.GG stats API endpoint (public, no auth needed)
	// Format: https://stats2.u.gg/lol/1.5/matchups/{patch}/{queue}/{region}/{rank}/{champion}/{role}.json
	// Using current patch, ranked solo, world, platinum+

	// Try the overview stats endpoint
	url := fmt.Sprintf("https://stats2.u.gg/lol/1.5/overview/15_1/ranked_solo_5x5/world/platinum_plus/%s/1.5.0.json", champion)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("stats API error: %d", resp.StatusCode)
	}

	// Parse the JSON response - structure varies, this is a simplified version
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	// Try to extract matchup data from response
	// The actual structure depends on U.GG's API format
	return nil, fmt.Errorf("parsing web API not implemented - try GraphQL")
}

func normalizeChampionName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, ".", "")
	// Special cases
	if name == "monkeyking" {
		return "wukong"
	}
	return name
}

func normalizeLane(lane string) string {
	lane = strings.ToLower(lane)
	switch lane {
	case "top":
		return "top"
	case "mid", "middle":
		return "mid"
	case "adc", "bot", "bottom":
		return "adc"
	case "jungle", "jg", "jung":
		return "jungle"
	case "support", "supp", "sup":
		return "support"
	}
	return ""
}

// Champion ID mappings (subset - add more as needed)
var championIDs = map[string]int{
	"aatrox": 266, "ahri": 103, "akali": 84, "akshan": 166, "alistar": 12,
	"amumu": 32, "anivia": 34, "annie": 1, "aphelios": 523, "ashe": 22,
	"aurelionsol": 136, "aurora": 893, "azir": 268, "bard": 432, "belveth": 200,
	"blitzcrank": 53, "brand": 63, "braum": 201, "briar": 233, "caitlyn": 51,
	"camille": 164, "cassiopeia": 69, "chogath": 31, "corki": 42, "darius": 122,
	"diana": 131, "draven": 119, "drmundo": 36, "ekko": 245, "elise": 60,
	"evelynn": 28, "ezreal": 81, "fiddlesticks": 9, "fiora": 114, "fizz": 105,
	"galio": 3, "gangplank": 41, "garen": 86, "gnar": 150, "gragas": 79,
	"graves": 104, "gwen": 887, "hecarim": 120, "heimerdinger": 74, "hwei": 910,
	"illaoi": 420, "irelia": 39, "ivern": 427, "janna": 40, "jarvaniv": 59,
	"jax": 24, "jayce": 126, "jhin": 202, "jinx": 222, "kaisa": 145,
	"kalista": 429, "karma": 43, "karthus": 30, "kassadin": 38, "katarina": 55,
	"kayle": 10, "kayn": 141, "kennen": 85, "khazix": 121, "kindred": 203,
	"kled": 240, "kogmaw": 96, "ksante": 897, "leblanc": 7, "leesin": 64,
	"leona": 89, "lillia": 876, "lissandra": 127, "lucian": 236, "lulu": 117,
	"lux": 99, "malphite": 54, "malzahar": 90, "maokai": 57, "masteryi": 11,
	"milio": 902, "missfortune": 21, "mordekaiser": 82, "morgana": 25, "naafiri": 950,
	"nami": 267, "nasus": 75, "nautilus": 111, "neeko": 518, "nidalee": 76,
	"nilah": 895, "nocturne": 56, "nunu": 20, "olaf": 2, "orianna": 61,
	"ornn": 516, "pantheon": 80, "poppy": 78, "pyke": 555, "qiyana": 246,
	"quinn": 133, "rakan": 497, "rammus": 33, "reksai": 421, "rell": 526,
	"renata": 888, "renekton": 58, "rengar": 107, "riven": 92, "rumble": 68,
	"ryze": 13, "samira": 360, "sejuani": 113, "senna": 235, "seraphine": 147,
	"sett": 875, "shaco": 35, "shen": 98, "shyvana": 102, "singed": 27,
	"sion": 14, "sivir": 15, "skarner": 72, "smolder": 901, "sona": 37,
	"soraka": 16, "swain": 50, "sylas": 517, "syndra": 134, "tahmkench": 223,
	"taliyah": 163, "talon": 91, "taric": 44, "teemo": 17, "thresh": 412,
	"tristana": 18, "trundle": 48, "tryndamere": 23, "twistedfate": 4, "twitch": 29,
	"udyr": 77, "urgot": 6, "varus": 110, "vayne": 67, "veigar": 45,
	"velkoz": 161, "vex": 711, "vi": 254, "viego": 234, "viktor": 112,
	"vladimir": 8, "volibear": 106, "warwick": 19, "wukong": 62, "xayah": 498,
	"xerath": 101, "xinzhao": 5, "yasuo": 157, "yone": 777, "yorick": 83,
	"yuumi": 350, "zac": 154, "zed": 238, "zeri": 221, "ziggs": 115,
	"zilean": 26, "zoe": 142, "zyra": 143, "mel": 942,
}

// Reverse mapping
var championNames = map[int]string{}

func init() {
	for name, id := range championIDs {
		championNames[id] = strings.Title(name)
	}
	// Fix multi-word names
	championNames[64] = "Lee Sin"
	championNames[21] = "Miss Fortune"
	championNames[136] = "Aurelion Sol"
	championNames[59] = "Jarvan IV"
	championNames[36] = "Dr. Mundo"
	championNames[223] = "Tahm Kench"
	championNames[4] = "Twisted Fate"
	championNames[5] = "Xin Zhao"
	championNames[96] = "Kog'Maw"
	championNames[31] = "Cho'Gath"
	championNames[121] = "Kha'Zix"
	championNames[421] = "Rek'Sai"
	championNames[161] = "Vel'Koz"
	championNames[145] = "Kai'Sa"
	championNames[897] = "K'Sante"
}

func getChampionID(name string) int {
	return championIDs[name]
}

func getChampionName(id int) string {
	if name, ok := championNames[id]; ok {
		return name
	}
	return fmt.Sprintf("Champion#%d", id)
}
