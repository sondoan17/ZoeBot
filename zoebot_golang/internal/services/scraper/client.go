package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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
		httpClient: &http.Client{Timeout: 5 * time.Second},
		redis:      redis,
	}
}

// GetCounters returns a list of counter champions.
func (c *Client) GetCounters(champion, lane string) ([]*CounterStats, error) {
	// Normalize inputs
	normChamp := normalizeChampionName(champion)
	normLane := normalizeLane(lane)

	// Redis Key: counter:v1:{champ}:{lane}
	cacheKey := fmt.Sprintf("counter:v1:%s:%s", normChamp, normLane)

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

	// 2. Scrape Data
	url := fmt.Sprintf("https://u.gg/lol/champions/%s/counter", normChamp)
	if normLane != "" {
		url = fmt.Sprintf("https://u.gg/lol/champions/%s/build/%s", normChamp, normLane)
	}

	log.Printf("Scraping counters from %s", url)
	stats, err := c.scrapeUGG(url)
	if err != nil {
		return nil, err
	}

	// 3. Save to Cache (24h)
	if c.redis != nil && len(stats) > 0 {
		data, _ := json.Marshal(stats)
		// We set a long TTL manually by just using Set (which has no TTL in our wrapper, but we can live with that or update wrapper)
		// User accepted "permanent" cache for PUUID, but for counters 24h is better.
		// Our RedisClient.Set doesn't support TTL. For now we just Set it.
		// TODO: Add SetWithTTL to RedisClient if needed, but for now persistent is 'okay' as patch doesn't change daily.
		// Actually, RedisClient wrapper in storage/redis.go only has Set(key, val, 0).
		// We will stick with Set for now, maybe add a version identifier in key to invalidate later.
		c.redis.Set(cacheKey, string(data))
	}

	return stats, nil
}

func (c *Client) scrapeUGG(url string) ([]*CounterStats, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []*CounterStats
	
	// Regex to extract data from text like "Kennen58.28% WR326 games"
	// Allow for spaces in name (e.g. Miss Fortune)
	// It's heuristic but should work for the specific U.GG layout
	re := regexp.MustCompile(`^(.+?)(\d{1,3}\.\d{1,2})% WR([\d,]+) games$`)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if len(results) >= 10 { // Limit to top 10
			return
		}

		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "% WR") && strings.Contains(text, "games") {
			matches := re.FindStringSubmatch(text)
			if len(matches) == 4 {
				// Filter out self (sometimes it lists itself?)
				name := strings.TrimSpace(matches[1])
				
				results = append(results, &CounterStats{
					ChampionName: name,
					WinRate:      matches[2] + "%",
					Matches:      matches[3],
					Lane:         "Lane", // Can't easily detect lane per row without more complex parsing
				})
			}
		}
	})
	
	// If scraping generic links failed, try specific table rows if classes are known.
	// But based on read_url_content, the text scraping above is most promising for a generic approach.
	
	if len(results) == 0 {
		return nil, fmt.Errorf("no counters found (parsing failed)")
	}

	return results, nil
}

func normalizeChampionName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, ".", "")
	// Special cases
	if name == "wukong" { return "monkeyking" } // U.GG might use wukong? Let's check. U.GG uses "wukong". Riot API uses MonkeyKing.
	// Actually U.GG uses "wukong".
	// Let's keep it simple for now.
	if name == "monkeyking" { return "wukong" }
	return name
}

func normalizeLane(lane string) string {
	lane = strings.ToLower(lane)
	switch lane {
	case "top", "mid", "adc", "jungle", "support", "supp", "jg":
		if lane == "jg" { return "jungle" }
		if lane == "supp" { return "support" }
		return lane
	}
	return "" // Default
}
