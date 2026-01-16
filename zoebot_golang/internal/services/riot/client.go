// Package riot provides Riot API client for ZoeBot.
package riot

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zoebot/internal/config"
	"github.com/zoebot/internal/storage"
)

// Client is a client for Riot Games API.
type Client struct {
	apiKey          string
	baseURLAccount  string
	baseURLMatch    string
	baseURLPlatform string // For summoner/league APIs
	httpClient      *http.Client
	championData    map[string]ChampionInfo
	redisClient     *storage.RedisClient
}

// NewClient creates a new Riot API client.
// Optimized: shorter timeout, connection reuse
func NewClient(cfg *config.Config, redisClient *storage.RedisClient) *Client {
	// Reuse connections for efficiency
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}

	c := &Client{
		apiKey:          cfg.RiotAPIKey,
		baseURLAccount:  cfg.RiotBaseURLAccount,
		baseURLMatch:    cfg.RiotBaseURLMatch,
		baseURLPlatform: cfg.RiotBaseURLPlatform,
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
		},
		championData: make(map[string]ChampionInfo),
		redisClient:  redisClient,
	}

	// Load champion data
	c.loadChampionData(cfg.ChampionDataPath())

	return c
}

// loadChampionData loads champion data from JSON file.
func (c *Client) loadChampionData(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	var data ChampionData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return
	}

	c.championData = data.Data
}

// GetChampionInfo returns champion tags and stats.
func (c *Client) GetChampionInfo(championName string) ([]string, int) {
	if champ, ok := c.championData[championName]; ok {
		return champ.Tags, champ.Info.Defense
	}
	return []string{}, 5
}

// doRequest makes an HTTP request to Riot API.
func (c *Client) doRequest(reqURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Riot-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// GetPUUIDByRiotID gets PUUID from Riot ID (Name#Tag).
// Uses Redis cache to avoid repeated API calls.
func (c *Client) GetPUUIDByRiotID(gameName, tagLine string) (string, error) {
	// Create cache key (lowercase for consistency)
	cacheKey := fmt.Sprintf("puuid:%s#%s", strings.ToLower(gameName), strings.ToLower(tagLine))

	// Check cache first
	if c.redisClient != nil {
		if cached, err := c.redisClient.Get(cacheKey); err == nil && cached != "" {
			log.Printf("PUUID cache hit for %s#%s", gameName, tagLine)
			return cached, nil
		}
	}

	reqURL := fmt.Sprintf("%s/riot/account/v1/accounts/by-riot-id/%s/%s",
		c.baseURLAccount,
		url.PathEscape(gameName),
		url.PathEscape(tagLine),
	)

	body, err := c.doRequest(reqURL)
	if err != nil {
		log.Printf("Error fetching PUUID for %s#%s: %v", gameName, tagLine, err)
		return "", err
	}

	var resp AccountResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// DEBUG LOGS
	if c.redisClient == nil {
		log.Println("DEBUG: redisClient is nil")
	} else {
		log.Printf("DEBUG: redisClient is set. Saving PUUID %s to key %s", resp.PUUID, cacheKey)
		// Save to cache (permanent storage)
		if resp.PUUID != "" {
			if err := c.redisClient.Set(cacheKey, resp.PUUID); err != nil {
				log.Printf("ERROR: Failed to cache PUUID: %v", err)
			} else {
				log.Printf("SUCCESS: PUUID cached for %s#%s", gameName, tagLine)
			}
		} else {
			log.Println("DEBUG: PUUID is empty, skipping cache")
		}
	}

	return resp.PUUID, nil
}

// GetMatchIDsByPUUID gets list of recent match IDs.
func (c *Client) GetMatchIDsByPUUID(puuid string, count int) ([]string, error) {
	reqURL := fmt.Sprintf("%s/lol/match/v5/matches/by-puuid/%s/ids?start=0&count=%d",
		c.baseURLMatch,
		puuid,
		count,
	)

	body, err := c.doRequest(reqURL)
	if err != nil {
		log.Printf("Error fetching match IDs for %s: %v", puuid, err)
		return nil, err
	}

	var matchIDs []string
	if err := json.Unmarshal(body, &matchIDs); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return matchIDs, nil
}

// GetSummonerByPUUID gets summoner data from PUUID.
func (c *Client) GetSummonerByPUUID(puuid string) (*SummonerDTO, error) {
	// Check cache
	cacheKey := fmt.Sprintf("summoner:%s", puuid)
	if c.redisClient != nil {
		if cached, err := c.redisClient.Get(cacheKey); err == nil && cached != "" {
			var summoner SummonerDTO
			if err := json.Unmarshal([]byte(cached), &summoner); err == nil {
				return &summoner, nil
			}
		}
	}

	// Use platform URL, fallback to match URL if not set
	baseURL := c.baseURLPlatform
	if baseURL == "" {
		baseURL = "https://vn2.api.riotgames.com"
	}

	reqURL := fmt.Sprintf("%s/lol/summoner/v4/summoners/by-puuid/%s",
		baseURL,
		puuid,
	)

	log.Printf("Fetching summoner from: %s", reqURL)

	body, err := c.doRequest(reqURL)
	if err != nil {
		return nil, err
	}

	var summoner SummonerDTO
	if err := json.Unmarshal(body, &summoner); err != nil {
		return nil, fmt.Errorf("failed to parse summoner response: %w", err)
	}

	// Cache for 1 hour (summoner data rarely changes)
	if c.redisClient != nil && summoner.ID != "" {
		data, _ := json.Marshal(summoner)
		c.redisClient.Set(cacheKey, string(data))
	}

	return &summoner, nil
}

// GetLeagueEntriesByPUUID gets ranked entries directly by PUUID.
func (c *Client) GetLeagueEntriesByPUUID(puuid string) ([]LeagueEntryDTO, error) {
	// Check cache
	cacheKey := fmt.Sprintf("league:puuid:%s", puuid)
	if c.redisClient != nil {
		if cached, err := c.redisClient.Get(cacheKey); err == nil && cached != "" {
			var entries []LeagueEntryDTO
			if err := json.Unmarshal([]byte(cached), &entries); err == nil {
				log.Printf("League cache hit for PUUID %s", puuid[:8])
				return entries, nil
			}
		}
	}

	// Use platform URL
	baseURL := c.baseURLPlatform
	if baseURL == "" {
		baseURL = "https://vn2.api.riotgames.com"
	}

	reqURL := fmt.Sprintf("%s/lol/league/v4/entries/by-puuid/%s",
		baseURL,
		puuid,
	)

	log.Printf("Fetching league entries by PUUID from: %s", reqURL)

	body, err := c.doRequest(reqURL)
	if err != nil {
		return nil, err
	}

	var entries []LeagueEntryDTO
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse league entries: %w", err)
	}

	// Cache for 10 minutes
	if c.redisClient != nil {
		data, _ := json.Marshal(entries)
		c.redisClient.Set(cacheKey, string(data))
	}

	return entries, nil
}

// GetPlayerRankInfo gets complete rank info for a player.
// Uses direct PUUID to League API endpoint.
func (c *Client) GetPlayerRankInfo(puuid, name string) (*PlayerRankInfo, error) {
	if puuid == "" {
		return nil, fmt.Errorf("empty PUUID")
	}

	// Use direct PUUID endpoint (no need for Summoner API)
	entries, err := c.GetLeagueEntriesByPUUID(puuid)
	if err != nil {
		log.Printf("League API failed for %s: %v", name, err)
		// Fallback to match history
		return c.buildRankInfoFromMatches(puuid, name)
	}

	return c.buildRankInfoFromLeague(puuid, name, entries), nil
}

// buildRankInfoFromLeague builds rank info from league entries.
func (c *Client) buildRankInfoFromLeague(puuid, name string, entries []LeagueEntryDTO) *PlayerRankInfo {
	var soloEntry, flexEntry *LeagueEntryDTO
	for i := range entries {
		switch entries[i].QueueType {
		case "RANKED_SOLO_5x5":
			soloEntry = &entries[i]
		case "RANKED_FLEX_SR":
			flexEntry = &entries[i]
		}
	}

	// Prioritize solo queue, fallback to flex
	entry := soloEntry
	if entry == nil {
		entry = flexEntry
	}

	info := &PlayerRankInfo{
		Name:  name,
		PUUID: puuid,
	}

	if entry == nil {
		info.Tier = "UNRANKED"
		info.QueueType = ""
		info.TierValue = 0
		return info
	}

	info.Tier = entry.Tier
	info.Rank = entry.Rank
	info.LP = entry.LeaguePoints
	info.Wins = entry.Wins
	info.Losses = entry.Losses
	info.TotalGames = entry.Wins + entry.Losses
	info.HotStreak = entry.HotStreak
	info.QueueType = entry.QueueType
	info.TierValue = TierOrder[entry.Tier]*100 + RankOrder[entry.Rank]*10 + entry.LeaguePoints/10

	if info.TotalGames > 0 {
		info.WinRate = float64(entry.Wins) / float64(info.TotalGames) * 100
	}

	return info
}

// buildRankInfoFromMatches calculates stats from recent match history.
func (c *Client) buildRankInfoFromMatches(puuid, name string) (*PlayerRankInfo, error) {
	// Get recent matches (last 20)
	matchIDs, err := c.GetMatchIDsByPUUID(puuid, 20)
	if err != nil || len(matchIDs) == 0 {
		return &PlayerRankInfo{
			Name:      name,
			PUUID:     puuid,
			Tier:      "UNRANKED",
			TierValue: 0,
		}, nil
	}

	wins := 0
	totalGames := 0

	// Check up to 10 matches for performance
	checkCount := min(10, len(matchIDs))
	for i := 0; i < checkCount; i++ {
		match, err := c.GetMatchDetails(matchIDs[i])
		if err != nil {
			continue
		}

		// Find player in match
		for _, p := range match.Info.Participants {
			if p.PUUID == puuid {
				totalGames++
				if p.Win {
					wins++
				}
				break
			}
		}
	}

	info := &PlayerRankInfo{
		Name:       name,
		PUUID:      puuid,
		Tier:       "N/A", // Can't determine tier from matches
		Wins:       wins,
		Losses:     totalGames - wins,
		TotalGames: totalGames,
		TierValue:  wins * 10, // Sort by wins
	}

	if totalGames > 0 {
		info.WinRate = float64(wins) / float64(totalGames) * 100
	}

	return info, nil
}

// GetMatchDetails gets full details of a match.
func (c *Client) GetMatchDetails(matchID string) (*MatchResponse, error) {
	reqURL := fmt.Sprintf("%s/lol/match/v5/matches/%s", c.baseURLMatch, matchID)

	body, err := c.doRequest(reqURL)
	if err != nil {
		log.Printf("Error fetching details for match %s: %v", matchID, err)
		return nil, err
	}

	var resp MatchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// GetMatchTimeline gets timeline data for a match.
func (c *Client) GetMatchTimeline(matchID string) (*TimelineResponse, error) {
	reqURL := fmt.Sprintf("%s/lol/match/v5/matches/%s/timeline", c.baseURLMatch, matchID)

	body, err := c.doRequest(reqURL)
	if err != nil {
		log.Printf("Error fetching timeline for match %s: %v", matchID, err)
		return nil, err
	}

	var resp TimelineResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// ParseMatchData filters and extracts data for AI analysis.
func (c *Client) ParseMatchData(match *MatchResponse, targetPUUID string, timeline *TimelineResponse) *ParsedMatchData {
	if match == nil {
		return nil
	}

	info := match.Info
	gameDuration := info.GameDuration
	gameDurationMinutes := float64(gameDuration) / 60.0
	if gameDurationMinutes == 0 {
		gameDurationMinutes = 1
	}

	// Find target player's team
	var targetTeamID int
	var targetPlayer *PlayerData

	for _, p := range info.Participants {
		if p.PUUID == targetPUUID {
			targetTeamID = p.TeamID
			break
		}
	}

	if targetTeamID == 0 {
		log.Println("Target player not found in match participants")
		return nil
	}

	var teammates []PlayerData
	var enemies []PlayerData

	for _, p := range info.Participants {
		pd := c.extractPlayerData(p, gameDurationMinutes)

		if p.TeamID == targetTeamID {
			teammates = append(teammates, pd)
			if p.PUUID == targetPUUID {
				targetPlayer = &pd
			}
		} else {
			enemies = append(enemies, pd)
		}
	}

	// Build lane matchups
	var laneMatchups []LaneMatchup
	for i := range teammates {
		teammate := &teammates[i]
		var opponent *PlayerData

		for j := range enemies {
			if enemies[j].TeamPosition == teammate.TeamPosition {
				opponent = &enemies[j]
				break
			}
		}

		laneMatchups = append(laneMatchups, LaneMatchup{
			Player:   teammate,
			Opponent: opponent,
		})
	}

	// Parse timeline if provided
	var timelineInsights *TimelineData
	if timeline != nil {
		timelineInsights = c.parseTimelineData(timeline, targetPUUID, targetTeamID, info.Participants)
	}

	var win bool
	var targetName string
	if targetPlayer != nil {
		win = targetPlayer.Win
		targetName = targetPlayer.RiotIDGameName
	}

	return &ParsedMatchData{
		MatchID:             match.Metadata.MatchID,
		GameDuration:        gameDuration,
		GameDurationMinutes: math.Round(gameDurationMinutes*10) / 10,
		GameMode:            info.GameMode,
		Win:                 win,
		TargetPlayerName:    targetName,
		Teammates:           teammates,
		LaneMatchups:        laneMatchups,
		TimelineInsights:    timelineInsights,
	}
}

// extractPlayerData extracts player data from participant.
func (c *Client) extractPlayerData(p Participant, gameDurationMinutes float64) PlayerData {
	tags, defense := c.GetChampionInfo(p.ChampionName)
	totalCS := p.TotalMinionsKilled + p.NeutralMinionsKilled

	return PlayerData{
		ChampionName:       p.ChampionName,
		ChampionTags:       tags,
		ChampionDefense:    defense,
		RiotIDGameName:     p.RiotIDGameName,
		TeamPosition:       p.TeamPosition,
		IndividualPosition: p.IndividualPosition,
		Win:                p.Win,

		// Combat
		Kills:               p.Kills,
		Deaths:              p.Deaths,
		Assists:             p.Assists,
		KDA:                 math.Round(p.Challenges.KDA*100) / 100,
		KillParticipation:   math.Round(p.Challenges.KillParticipation*1000) / 10,
		Takedowns:           p.Challenges.Takedowns,
		LargestKillingSpree: p.LargestKillingSpree,
		SoloKills:           p.Challenges.SoloKills,
		TimeSpentDead:       p.TotalTimeSpentDead,

		// Damage
		TotalDamageDealtToChampions: p.TotalDamageDealtToChampions,
		DamagePerMinute:             math.Round(p.Challenges.DamagePerMinute),
		TeamDamagePercentage:        math.Round(p.Challenges.TeamDamagePercentage*1000) / 10,
		TimeCCingOthers:             p.TimeCCingOthers,
		TotalDamageTaken:            p.TotalDamageTaken,
		DamageTakenOnTeamPct:        math.Round(p.Challenges.DamageTakenOnTeamPct*1000) / 10,
		DamageSelfMitigated:         p.DamageSelfMitigated,

		// Economy
		LaneMinionsFirst10Min: p.Challenges.LaneMinionsFirst10Min,
		TotalCS:               totalCS,
		CSPerMinute:           math.Round(float64(totalCS)/gameDurationMinutes*10) / 10,
		GoldEarned:            p.GoldEarned,
		GoldPerMinute:         math.Round(p.Challenges.GoldPerMinute),
		ChampLevel:            p.ChampLevel,

		// Objectives
		DragonTakedowns:         p.Challenges.DragonTakedowns,
		BaronTakedowns:          p.Challenges.BaronTakedowns,
		DamageDealtToObjectives: p.DamageDealtToObjectives,
		TurretTakedowns:         p.Challenges.TurretTakedowns,

		// Vision
		VisionScore:          p.VisionScore,
		VisionScorePerMinute: math.Round(p.Challenges.VisionScorePerMinute*100) / 100,
		WardsPlaced:          p.WardsPlaced,
		ControlWardsPlaced:   p.Challenges.ControlWardsPlaced,
		WardsKilled:          p.WardsKilled,
	}
}

// parseTimelineData parses timeline data for key insights.
func (c *Client) parseTimelineData(timeline *TimelineResponse, targetPUUID string, targetTeamID int, participants []Participant) *TimelineData {
	if timeline == nil {
		return nil
	}

	// Build ID mappings
	puuidToID := make(map[string]int)
	idToName := make(map[int]string)
	idToPosition := make(map[int]string)
	idToTeam := make(map[int]int)

	for _, p := range participants {
		puuidToID[p.PUUID] = p.ParticipantID
		idToName[p.ParticipantID] = p.RiotIDGameName
		idToPosition[p.ParticipantID] = p.TeamPosition
		idToTeam[p.ParticipantID] = p.TeamID
	}

	var deathsTimeline []DeathInfo
	var killsTimeline []KillInfo
	var firstBlood *KillInfo
	var objectiveKills []ObjectiveKill
	var platesDestroyed, platesLost int

	goldSnapshots := make(map[string]map[string]int)
	keyMinutes := []int{5, 10, 15}

	for _, frame := range timeline.Info.Frames {
		frameMin := float64(frame.Timestamp) / 1000 / 60

		// Check for key minute snapshots
		for _, targetMin := range keyMinutes {
			if math.Abs(frameMin-float64(targetMin)) < 0.5 {
				key := strconv.Itoa(targetMin) + "min"
				if _, exists := goldSnapshots[key]; !exists {
					goldSnapshots[key] = make(map[string]int)
					for pidStr, pf := range frame.ParticipantFrames {
						pid, _ := strconv.Atoi(pidStr)
						name := idToName[pid]
						goldSnapshots[key][name] = pf.TotalGold
					}
				}
			}
		}

		// Process events
		for _, event := range frame.Events {
			eventTimeMin := math.Round(float64(event.Timestamp)/1000/60*10) / 10

			switch event.Type {
			case "CHAMPION_KILL":
				killInfo := KillInfo{
					TimeMin:        eventTimeMin,
					Killer:         idToName[event.KillerID],
					KillerID:       event.KillerID,
					Victim:         idToName[event.VictimID],
					VictimID:       event.VictimID,
					Bounty:         event.Bounty,
					ShutdownBounty: event.ShutdownBounty,
					KillStreak:     event.KillStreakLength,
				}

				for _, assistID := range event.AssistingParticipantIDs {
					killInfo.Assists = append(killInfo.Assists, idToName[assistID])
				}

				if firstBlood == nil {
					firstBlood = &killInfo
				}

				if idToTeam[event.VictimID] == targetTeamID {
					deathsTimeline = append(deathsTimeline, DeathInfo{
						TimeMin:  eventTimeMin,
						Player:   idToName[event.VictimID],
						Position: idToPosition[event.VictimID],
						Killer:   idToName[event.KillerID],
					})
				}

				if idToTeam[event.KillerID] == targetTeamID && len(killsTimeline) < 10 {
					killsTimeline = append(killsTimeline, killInfo)
				}

			case "ELITE_MONSTER_KILL":
				objectiveKills = append(objectiveKills, ObjectiveKill{
					TimeMin:        eventTimeMin,
					MonsterType:    event.MonsterType,
					MonsterSubType: event.MonsterSubType,
					Killer:         idToName[event.KillerID],
					KillerTeam:     idToTeam[event.KillerID],
				})

			case "TURRET_PLATE_DESTROYED":
				if event.TeamID != targetTeamID {
					platesDestroyed++
				} else {
					platesLost++
				}
			}
		}
	}

	// Calculate gold diff at 10min
	goldDiff10Min := make(map[string]GoldDiff)
	if gold10, ok := goldSnapshots["10min"]; ok {
		for _, p := range participants {
			if p.TeamID == targetTeamID {
				playerGold := gold10[p.RiotIDGameName]

				for _, opp := range participants {
					if opp.TeamID != targetTeamID && opp.TeamPosition == p.TeamPosition {
						oppGold := gold10[opp.RiotIDGameName]
						goldDiff10Min[p.RiotIDGameName] = GoldDiff{
							Gold:         playerGold,
							OpponentGold: oppGold,
							Diff:         playerGold - oppGold,
							Position:     p.TeamPosition,
						}
						break
					}
				}
			}
		}
	}

	// Count deaths before 10min
	deathsBy10 := 0
	for _, d := range deathsTimeline {
		if d.TimeMin <= 10 {
			deathsBy10++
		}
	}

	return &TimelineData{
		FirstBlood:            firstBlood,
		DeathsTimeline:        deathsTimeline,
		KillsTimeline:         killsTimeline,
		ObjectiveKills:        objectiveKills,
		TurretPlatesDestroyed: platesDestroyed,
		TurretPlatesLost:      platesLost,
		GoldDiff10Min:         goldDiff10Min,
		TotalTeamDeathsBy10:   deathsBy10,
	}
}
