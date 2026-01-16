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
		httpClient: &http.Client{Timeout: 10 * time.Second},
		redis:      redis,
	}
}

// GetCounters returns counter data including best and worst picks.
func (c *Client) GetCounters(champion, lane string) (*CounterData, error) {
	// Normalize inputs
	normChamp := normalizeChampionName(champion)
	normLane := normalizeLane(lane)

	// Redis Key: counter:v4:{champ}:{lane}
	cacheKey := fmt.Sprintf("counter:v4:%s:%s", normChamp, normLane)

	// 1. Check Cache
	if c.redis != nil {
		if val, err := c.redis.Get(cacheKey); err == nil && val != "" {
			var data CounterData
			if err := json.Unmarshal([]byte(val), &data); err == nil {
				log.Printf("Counter cache hit for %s %s", champion, lane)
				return &data, nil
			}
		}
	}

	// 2. Scrape from CounterStats.net
	url := fmt.Sprintf("https://counterstats.net/league-of-legends/%s", normChamp)
	log.Printf("Scraping counters from %s", url)

	data, err := c.scrapeCounterStats(url, normLane)
	if err != nil {
		return nil, err
	}

	// 3. Save to Cache
	if c.redis != nil && (len(data.BestPicks) > 0 || len(data.WorstPicks) > 0) {
		jsonData, _ := json.Marshal(data)
		c.redis.Set(cacheKey, string(jsonData))
	}

	return data, nil
}

// scrapeCounterStats scrapes counter data from counterstats.net
func (c *Client) scrapeCounterStats(url, lane string) (*CounterData, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

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

	result := &CounterData{}
	targetLane := strings.ToLower(lane)

	// Find the correct lane section
	doc.Find("div.champ-box__wrap").Each(func(i int, section *goquery.Selection) {
		h2Text := strings.ToLower(section.Find("h2").First().Text())

		// If lane is specified, only parse that lane's section
		if targetLane != "" {
			if !strings.Contains(h2Text, targetLane) {
				return
			}
		}

		// Already found data for this lane
		if len(result.BestPicks) > 0 || len(result.WorstPicks) > 0 {
			return
		}

		result.Lane = detectLaneFromHeader(h2Text)

		// Parse each champ-box (Best Picks and Worst Picks)
		section.Find("div.champ-box").Each(func(j int, box *goquery.Selection) {
			h3Text := strings.ToLower(box.Find("h3").Text())
			isBestPicks := strings.Contains(h3Text, "best picks")
			isWorstPicks := strings.Contains(h3Text, "worst picks")

			if !isBestPicks && !isWorstPicks {
				return
			}

			var picks []*CounterStats
			box.Find("a.champ-box__row").Each(func(k int, row *goquery.Selection) {
				if len(picks) >= 5 {
					return
				}

				// Check if row is visible
				style, exists := row.Attr("style")
				if exists && strings.Contains(style, "display:none") {
					return
				}

				champName := strings.TrimSpace(row.Find("span.champion").Text())
				if champName == "" {
					return
				}

				winRate := strings.TrimSpace(row.Find("span.win span.b").Text())
				if winRate == "" {
					winRate = strings.TrimSpace(row.Find("span.b").Text())
				}
				if winRate != "" && !strings.HasSuffix(winRate, "%") {
					winRate = winRate + "%"
				}

				picks = append(picks, &CounterStats{
					ChampionName: champName,
					WinRate:      winRate,
					Lane:         result.Lane,
				})
			})

			if isBestPicks {
				result.BestPicks = picks
			} else if isWorstPicks {
				result.WorstPicks = picks
			}
		})
	})

	if len(result.BestPicks) == 0 && len(result.WorstPicks) == 0 {
		return nil, fmt.Errorf("no counters found for this champion")
	}

	return result, nil
}

func detectLaneFromHeader(headerText string) string {
	headerText = strings.ToLower(headerText)
	switch {
	case strings.Contains(headerText, "mid"):
		return "Mid"
	case strings.Contains(headerText, "top"):
		return "Top"
	case strings.Contains(headerText, "jungle"):
		return "Jungle"
	case strings.Contains(headerText, "adc"), strings.Contains(headerText, "bot"):
		return "ADC"
	case strings.Contains(headerText, "support"):
		return "Support"
	}
	return ""
}

func normalizeChampionName(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSpace(name)

	specialCases := map[string]string{
		"aurelionsol":  "aurelion-sol",
		"aurelion sol": "aurelion-sol",
		"leesin":       "lee-sin",
		"lee sin":      "lee-sin",
		"missfortune":  "miss-fortune",
		"miss fortune": "miss-fortune",
		"jarvaniv":     "jarvan-iv",
		"jarvan iv":    "jarvan-iv",
		"jarvan4":      "jarvan-iv",
		"drmundo":      "dr-mundo",
		"dr mundo":     "dr-mundo",
		"dr.mundo":     "dr-mundo",
		"twistedfate":  "twisted-fate",
		"twisted fate": "twisted-fate",
		"tahmkench":    "tahm-kench",
		"tahm kench":   "tahm-kench",
		"xinzhao":      "xin-zhao",
		"xin zhao":     "xin-zhao",
		"kogmaw":       "kogmaw",
		"kog'maw":      "kogmaw",
		"chogath":      "chogath",
		"cho'gath":     "chogath",
		"khazix":       "khazix",
		"kha'zix":      "khazix",
		"reksai":       "reksai",
		"rek'sai":      "reksai",
		"velkoz":       "velkoz",
		"vel'koz":      "velkoz",
		"kaisa":        "kaisa",
		"kai'sa":       "kaisa",
		"ksante":       "ksante",
		"k'sante":      "ksante",
		"monkeyking":   "wukong",
		"belveth":      "belveth",
		"bel'veth":     "belveth",
		"renata":       "renata-glasc",
		"renataglasc":  "renata-glasc",
		"nunu":         "nunu",
		"nunu&willump": "nunu",
		"masteryi":     "master-yi",
		"master yi":    "master-yi",
	}

	cleanName := strings.ReplaceAll(name, "'", "")
	cleanName = strings.ReplaceAll(cleanName, ".", "")
	cleanName = strings.ReplaceAll(cleanName, " ", "")

	if mapped, ok := specialCases[name]; ok {
		return mapped
	}
	if mapped, ok := specialCases[cleanName]; ok {
		return mapped
	}

	return regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(name, "")
}

func normalizeLane(lane string) string {
	lane = strings.ToLower(strings.TrimSpace(lane))
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
