package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/zoebot/internal/data"
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
				return &data, nil
			}
		}
	}

	// 2. Scrape from CounterStats.net
	url := fmt.Sprintf("https://counterstats.net/league-of-legends/%s", normChamp)

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

// normalizeLaneForOPGG converts lane to OP.GG format.
func normalizeLaneForOPGG(lane string) string {
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

// normalizeChampionNameForOPGG converts champion name to OP.GG URL format.
func normalizeChampionNameForOPGG(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	// Special cases for OP.GG
	specialCases := map[string]string{
		"aurelionsol":    "aurelion-sol",
		"aurelion sol":   "aurelion-sol",
		"leesin":         "lee-sin",
		"lee sin":        "lee-sin",
		"missfortune":    "miss-fortune",
		"miss fortune":   "miss-fortune",
		"jarvaniv":       "jarvan-iv",
		"jarvan iv":      "jarvan-iv",
		"jarvan4":        "jarvan-iv",
		"drmundo":        "dr.-mundo",
		"dr mundo":       "dr.-mundo",
		"dr.mundo":       "dr.-mundo",
		"twistedfate":    "twisted-fate",
		"twisted fate":   "twisted-fate",
		"tahmkench":      "tahm-kench",
		"tahm kench":     "tahm-kench",
		"xinzhao":        "xin-zhao",
		"xin zhao":       "xin-zhao",
		"kogmaw":         "kog'maw",
		"kog'maw":        "kog'maw",
		"chogath":        "cho'gath",
		"cho'gath":       "cho'gath",
		"khazix":         "kha'zix",
		"kha'zix":        "kha'zix",
		"reksai":         "rek'sai",
		"rek'sai":        "rek'sai",
		"velkoz":         "vel'koz",
		"vel'koz":        "vel'koz",
		"kaisa":          "kai'sa",
		"kai'sa":         "kai'sa",
		"ksante":         "k'sante",
		"k'sante":        "k'sante",
		"monkeyking":     "wukong",
		"belveth":        "bel'veth",
		"bel'veth":       "bel'veth",
		"renata":         "renata-glasc",
		"renataglasc":    "renata-glasc",
		"renata glasc":   "renata-glasc",
		"nunu":           "nunu",
		"nunu&willump":   "nunu",
		"nunu & willump": "nunu",
		"masteryi":       "master-yi",
		"master yi":      "master-yi",
	}

	// Try exact match first
	if mapped, ok := specialCases[name]; ok {
		return mapped
	}

	// Try without special chars
	cleanName := strings.ReplaceAll(name, "'", "")
	cleanName = strings.ReplaceAll(cleanName, ".", "")
	cleanName = strings.ReplaceAll(cleanName, " ", "")

	if mapped, ok := specialCases[cleanName]; ok {
		return mapped
	}

	// Default: just return lowercase
	return regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(name, "")
}

// GetBuild scrapes champion build data from OP.GG.
func (c *Client) GetBuild(champion, role string) (*BuildData, error) {
	normChamp := normalizeChampionNameForOPGG(champion)
	normRole := normalizeLaneForOPGG(role)

	if normRole == "" {
		return nil, fmt.Errorf("invalid role: %s (use: top, jungle, mid, adc, support)", role)
	}

	// Redis Key: build:v2:{champ}:{role} (v2 = Vietnamese data)
	cacheKey := fmt.Sprintf("build:v2:%s:%s", normChamp, normRole)

	// 1. Check Cache
	if c.redis != nil {
		if val, err := c.redis.Get(cacheKey); err == nil && val != "" {
			var data BuildData
			if err := json.Unmarshal([]byte(val), &data); err == nil {
				return &data, nil
			}
		}
	}

	// 2. Scrape from OP.GG
	url := fmt.Sprintf("https://www.op.gg/champions/%s/build/%s", normChamp, normRole)

	data, err := c.scrapeOPGGBuild(url, champion, role)
	if err != nil {
		return nil, err
	}

	// 3. Save to Cache (10 minutes)
	if c.redis != nil && data != nil {
		jsonData, _ := json.Marshal(data)
		c.redis.Set(cacheKey, string(jsonData))
	}

	return data, nil
}

// scrapeOPGGBuild scrapes build data from OP.GG page.
func (c *Client) scrapeOPGGBuild(url, champion, role string) (*BuildData, error) {
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

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("champion '%s' not found on OP.GG for role '%s'", champion, role)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OP.GG returned status: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	result := &BuildData{
		Champion: champion,
		Role:     role,
	}

	// Extract patch version from URL in images
	doc.Find("img[src*='/meta/images/lol/']").First().Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			// Extract version like "16.01" from URL
			re := regexp.MustCompile(`/lol/(\d+\.\d+)/`)
			if matches := re.FindStringSubmatch(src); len(matches) > 1 {
				result.PatchVersion = matches[1]
			}
		}
	})

	// Extract Runes
	c.extractRunes(doc, result)

	// Extract Items
	c.extractItems(doc, result)

	if len(result.PrimaryRunes) == 0 && len(result.CoreItems) == 0 {
		return nil, fmt.Errorf("no build data found for %s %s", champion, role)
	}

	return result, nil
}

// extractRunes extracts rune data from the page.
func (c *Client) extractRunes(doc *goquery.Document, result *BuildData) {
	// Find rune section - look for tables with rune data
	// OP.GG uses images with src containing /perk/ for runes

	// Extract primary tree and runes
	var primaryRuneIDs []int
	var secondaryRuneIDs []int
	var statShardIDs []int

	// Find all selected runes (opacity-100, not grayscale)
	doc.Find("img[src*='/perk/']").Each(func(i int, s *goquery.Selection) {
		// Check if this rune is selected (has opacity-100 or bg-black class)
		parent := s.Parent()
		classes, _ := parent.Attr("class")
		imgClasses, _ := s.Attr("class")

		// Selected runes typically have opacity-100 and no grayscale
		isSelected := strings.Contains(imgClasses, "opacity-100") ||
			strings.Contains(classes, "bg-black") ||
			(!strings.Contains(imgClasses, "grayscale") && !strings.Contains(imgClasses, "opacity-50"))

		if !isSelected {
			return
		}

		src, _ := s.Attr("src")
		runeID := extractIDFromURL(src)
		if runeID > 0 {
			primaryRuneIDs = append(primaryRuneIDs, runeID)
		}
	})

	// Find perkStyle (tree) images
	doc.Find("img[src*='/perkStyle/']").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		treeID := extractIDFromURL(src)
		if treeID > 0 {
			if result.PrimaryTree == "" {
				if name := data.GetPerkStyleName(treeID); name != "" {
					result.PrimaryTree = name
				}
			} else if result.SecondaryTree == "" {
				if name := data.GetPerkStyleName(treeID); name != "" {
					result.SecondaryTree = name
				}
			}
		}
	})

	// Find stat shards
	doc.Find("img[src*='/perkShard/']").Each(func(i int, s *goquery.Selection) {
		// Check if selected (has golden border)
		imgClasses, _ := s.Attr("class")
		isSelected := strings.Contains(imgClasses, "border-[#bb9834]") ||
			(!strings.Contains(imgClasses, "grayscale"))

		if !isSelected {
			return
		}

		src, _ := s.Attr("src")
		shardID := extractIDFromURL(src)
		if shardID > 0 {
			statShardIDs = append(statShardIDs, shardID)
		}
	})

	// Convert IDs to names
	// Primary runes: first 4 (keystone + 3 minor)
	for i, id := range primaryRuneIDs {
		if i >= 4 {
			break
		}
		if name := data.GetPerkName(id); name != "" {
			result.PrimaryRunes = append(result.PrimaryRunes, name)
			// Save keystone ID (first rune)
			if i == 0 {
				result.KeystoneID = id
			}
		}
	}

	// Secondary runes: next 2
	for i := 4; i < len(primaryRuneIDs) && i < 6; i++ {
		if name := data.GetPerkName(primaryRuneIDs[i]); name != "" {
			secondaryRuneIDs = append(secondaryRuneIDs, primaryRuneIDs[i])
			result.SecondaryRunes = append(result.SecondaryRunes, name)
		}
	}

	// Stat shards: first 3
	for i, id := range statShardIDs {
		if i >= 3 {
			break
		}
		if name := data.GetPerkName(id); name != "" {
			result.StatShards = append(result.StatShards, name)
		}
	}

	// Extract rune win rate from the table
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		caption := table.Find("caption").Text()
		if strings.Contains(strings.ToLower(caption), "rune") {
			// Find win rate in this table
			table.Find("strong.text-main-600, strong.text-blue-500").First().Each(func(j int, s *goquery.Selection) {
				result.RuneWinRate = strings.TrimSpace(s.Text())
			})
			// Find pick rate
			table.Find("td").Each(func(j int, td *goquery.Selection) {
				text := strings.TrimSpace(td.Text())
				if strings.Contains(text, "%") && result.RunePickRate == "" && text != result.RuneWinRate {
					// This might be pick rate
					td.Find("strong").First().Each(func(k int, s *goquery.Selection) {
						result.RunePickRate = strings.TrimSpace(s.Text())
					})
				}
			})
		}
	})
}

// extractItems extracts item build data from the page.
func (c *Client) extractItems(doc *goquery.Document, result *BuildData) {
	// Find all item images
	var allItemIDs []int

	doc.Find("img[src*='/item/']").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		itemID := extractIDFromURL(src)
		if itemID > 0 {
			allItemIDs = append(allItemIDs, itemID)
		}
	})

	// Look for tables to identify starter items, boots, core items
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		// Check table header/caption
		headerText := strings.ToLower(table.Find("caption, thead th").Text())

		// Find items in this table
		var tableItems []int
		table.Find("img[src*='/item/']").Each(func(j int, s *goquery.Selection) {
			src, _ := s.Attr("src")
			itemID := extractIDFromURL(src)
			if itemID > 0 {
				tableItems = append(tableItems, itemID)
			}
		})

		// Find win rate in this table
		var winRate, pickRate string
		table.Find("strong.text-main-600, strong.text-blue-500").First().Each(func(j int, s *goquery.Selection) {
			winRate = strings.TrimSpace(s.Text())
		})
		table.Find("td strong").Each(func(j int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if strings.Contains(text, "%") && text != winRate && pickRate == "" {
				pickRate = text
			}
		})

		if strings.Contains(headerText, "starter") || strings.Contains(headerText, "khởi đầu") || strings.Contains(headerText, "start") {
			// Starter items
			for _, id := range tableItems {
				result.StarterItems = append(result.StarterItems, fmt.Sprintf("%d", id))
			}
		} else if strings.Contains(headerText, "boot") || strings.Contains(headerText, "giày") {
			// Boots
			if len(tableItems) > 0 {
				result.Boots = fmt.Sprintf("%d", tableItems[0])
			}
		} else if strings.Contains(headerText, "core") || strings.Contains(headerText, "cốt lõi") || strings.Contains(headerText, "build") {
			// Core items
			for _, id := range tableItems {
				if len(result.CoreItems) < 3 {
					result.CoreItems = append(result.CoreItems, fmt.Sprintf("%d", id))
				}
			}
			if winRate != "" {
				result.ItemWinRate = winRate
			}
			if pickRate != "" {
				result.ItemPickRate = pickRate
			}
		}
	})

	// If we didn't find structured data, use first items found
	if len(result.CoreItems) == 0 && len(allItemIDs) > 0 {
		// Skip first few (likely starter items) and take next 3-4 as core
		start := 2
		if len(allItemIDs) < 5 {
			start = 0
		}
		for i := start; i < len(allItemIDs) && len(result.CoreItems) < 3; i++ {
			result.CoreItems = append(result.CoreItems, fmt.Sprintf("%d", allItemIDs[i]))
		}
	}
}

// extractIDFromURL extracts numeric ID from OP.GG static URL.
// Example: https://opgg-static.akamaized.net/meta/images/lol/16.01/perk/8214.png -> 8214
func extractIDFromURL(url string) int {
	re := regexp.MustCompile(`/(\d+)\.png`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		var id int
		fmt.Sscanf(matches[1], "%d", &id)
		return id
	}
	return 0
}
