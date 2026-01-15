// Package riot provides types for Riot API responses.
package riot

// AccountResponse represents the response from Riot Account API.
type AccountResponse struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

// MatchInfo represents the info section of a match response.
type MatchInfo struct {
	GameDuration int64         `json:"gameDuration"`
	GameMode     string        `json:"gameMode"`
	Participants []Participant `json:"participants"`
}

// MatchResponse represents the full match response from Riot API.
type MatchResponse struct {
	Metadata struct {
		MatchID string `json:"matchId"`
	} `json:"metadata"`
	Info MatchInfo `json:"info"`
}

// Participant represents a player in a match.
type Participant struct {
	PUUID              string     `json:"puuid"`
	ParticipantID      int        `json:"participantId"`
	RiotIDGameName     string     `json:"riotIdGameName"`
	ChampionName       string     `json:"championName"`
	TeamID             int        `json:"teamId"`
	TeamPosition       string     `json:"teamPosition"`
	IndividualPosition string     `json:"individualPosition"`
	Win                bool       `json:"win"`
	Kills              int        `json:"kills"`
	Deaths             int        `json:"deaths"`
	Assists            int        `json:"assists"`
	ChampLevel         int        `json:"champLevel"`
	LargestKillingSpree int       `json:"largestKillingSpree"`
	TotalTimeSpentDead  int       `json:"totalTimeSpentDead"`

	// Damage
	TotalDamageDealtToChampions int `json:"totalDamageDealtToChampions"`
	TotalDamageTaken            int `json:"totalDamageTaken"`
	DamageSelfMitigated         int `json:"damageSelfMitigated"`
	TimeCCingOthers             int `json:"timeCCingOthers"`
	DamageDealtToObjectives     int `json:"damageDealtToObjectives"`

	// CS and Gold
	TotalMinionsKilled   int `json:"totalMinionsKilled"`
	NeutralMinionsKilled int `json:"neutralMinionsKilled"`
	GoldEarned           int `json:"goldEarned"`

	// Vision
	VisionScore       int `json:"visionScore"`
	WardsPlaced       int `json:"wardsPlaced"`
	WardsKilled       int `json:"wardsKilled"`

	// Challenges (nested stats)
	Challenges Challenges `json:"challenges"`
}

// Challenges represents the challenges/stats section of a participant.
type Challenges struct {
	KDA                       float64 `json:"kda"`
	KillParticipation         float64 `json:"killParticipation"`
	Takedowns                 int     `json:"takedowns"`
	SoloKills                 int     `json:"soloKills"`
	DamagePerMinute           float64 `json:"damagePerMinute"`
	TeamDamagePercentage      float64 `json:"teamDamagePercentage"`
	DamageTakenOnTeamPct      float64 `json:"damageTakenOnTeamPercentage"`
	LaneMinionsFirst10Min     int     `json:"laneMinionsFirst10Minutes"`
	GoldPerMinute             float64 `json:"goldPerMinute"`
	DragonTakedowns           int     `json:"dragonTakedowns"`
	BaronTakedowns            int     `json:"baronTakedowns"`
	TurretTakedowns           int     `json:"turretTakedowns"`
	VisionScorePerMinute      float64 `json:"visionScorePerMinute"`
	ControlWardsPlaced        int     `json:"controlWardsPlaced"`
}

// TimelineResponse represents the timeline response from Riot API.
type TimelineResponse struct {
	Info TimelineInfo `json:"info"`
}

// TimelineInfo represents the info section of a timeline response.
type TimelineInfo struct {
	Frames []TimelineFrame `json:"frames"`
}

// TimelineFrame represents a frame in the timeline.
type TimelineFrame struct {
	Timestamp         int64                        `json:"timestamp"`
	Events            []TimelineEvent              `json:"events"`
	ParticipantFrames map[string]ParticipantFrame  `json:"participantFrames"`
}

// TimelineEvent represents an event in the timeline.
type TimelineEvent struct {
	Type                      string `json:"type"`
	Timestamp                 int64  `json:"timestamp"`
	KillerID                  int    `json:"killerId"`
	VictimID                  int    `json:"victimId"`
	AssistingParticipantIDs   []int  `json:"assistingParticipantIds"`
	Bounty                    int    `json:"bounty"`
	ShutdownBounty            int    `json:"shutdownBounty"`
	KillStreakLength          int    `json:"killStreakLength"`
	MonsterType               string `json:"monsterType"`
	MonsterSubType            string `json:"monsterSubType"`
	LaneType                  string `json:"laneType"`
	TeamID                    int    `json:"teamId"`
}

// ParticipantFrame represents a participant's state at a frame.
type ParticipantFrame struct {
	ParticipantID       int `json:"participantId"`
	TotalGold           int `json:"totalGold"`
	MinionsKilled       int `json:"minionsKilled"`
	JungleMinionsKilled int `json:"jungleMinionsKilled"`
}

// ChampionData represents champion data from Data Dragon.
type ChampionData struct {
	Data map[string]ChampionInfo `json:"data"`
}

// ChampionInfo represents info about a single champion.
type ChampionInfo struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
	Info struct {
		Attack  int `json:"attack"`
		Defense int `json:"defense"`
		Magic   int `json:"magic"`
	} `json:"info"`
}

// ParsedMatchData represents processed match data for AI analysis.
type ParsedMatchData struct {
	MatchID             string          `json:"matchId"`
	GameDuration        int64           `json:"gameDuration"`
	GameDurationMinutes float64         `json:"gameDurationMinutes"`
	GameMode            string          `json:"gameMode"`
	Win                 bool            `json:"win"`
	TargetPlayerName    string          `json:"target_player_name"`
	Teammates           []PlayerData    `json:"teammates"`
	LaneMatchups        []LaneMatchup   `json:"lane_matchups"`
	TimelineInsights    *TimelineData   `json:"timeline_insights,omitempty"`
}

// PlayerData represents processed player data.
type PlayerData struct {
	ChampionName      string   `json:"championName"`
	ChampionTags      []string `json:"championTags"`
	ChampionDefense   int      `json:"championDefense"`
	RiotIDGameName    string   `json:"riotIdGameName"`
	TeamPosition      string   `json:"teamPosition"`
	IndividualPosition string  `json:"individualPosition"`
	Win               bool     `json:"win"`

	// Combat
	Kills              int     `json:"kills"`
	Deaths             int     `json:"deaths"`
	Assists            int     `json:"assists"`
	KDA                float64 `json:"kda"`
	KillParticipation  float64 `json:"killParticipation"`
	Takedowns          int     `json:"takedowns"`
	LargestKillingSpree int    `json:"largestKillingSpree"`
	SoloKills          int     `json:"soloKills"`
	TimeSpentDead      int     `json:"timeSpentDead"`

	// Damage
	TotalDamageDealtToChampions int     `json:"totalDamageDealtToChampions"`
	DamagePerMinute             float64 `json:"damagePerMinute"`
	TeamDamagePercentage        float64 `json:"teamDamagePercentage"`
	TimeCCingOthers             int     `json:"timeCCingOthers"`
	TotalDamageTaken            int     `json:"totalDamageTaken"`
	DamageTakenOnTeamPct        float64 `json:"damageTakenOnTeamPercentage"`
	DamageSelfMitigated         int     `json:"damageSelfMitigated"`

	// Economy
	LaneMinionsFirst10Min int     `json:"laneMinionsFirst10Minutes"`
	TotalCS               int     `json:"totalCS"`
	CSPerMinute           float64 `json:"csPerMinute"`
	GoldEarned            int     `json:"goldEarned"`
	GoldPerMinute         float64 `json:"goldPerMinute"`
	ChampLevel            int     `json:"champLevel"`

	// Objectives
	DragonTakedowns       int `json:"dragonTakedowns"`
	BaronTakedowns        int `json:"baronTakedowns"`
	DamageDealtToObjectives int `json:"damageDealtToObjectives"`
	TurretTakedowns       int `json:"turretTakedowns"`

	// Vision
	VisionScore          int     `json:"visionScore"`
	VisionScorePerMinute float64 `json:"visionScorePerMinute"`
	WardsPlaced          int     `json:"wardsPlaced"`
	ControlWardsPlaced   int     `json:"controlWardsPlaced"`
	WardsKilled          int     `json:"wardsKilled"`
}

// LaneMatchup represents a lane matchup between player and opponent.
type LaneMatchup struct {
	Player   *PlayerData `json:"player"`
	Opponent *PlayerData `json:"opponent,omitempty"`
}

// TimelineData represents processed timeline insights.
type TimelineData struct {
	FirstBlood            *KillInfo           `json:"first_blood,omitempty"`
	DeathsTimeline        []DeathInfo         `json:"deaths_timeline"`
	KillsTimeline         []KillInfo          `json:"kills_timeline"`
	ObjectiveKills        []ObjectiveKill     `json:"objective_kills"`
	TurretPlatesDestroyed int                 `json:"turret_plates_destroyed"`
	TurretPlatesLost      int                 `json:"turret_plates_lost"`
	GoldDiff10Min         map[string]GoldDiff `json:"gold_diff_10min"`
	TotalTeamDeathsBy10   int                 `json:"total_team_deaths_by_10min"`
}

// KillInfo represents info about a kill event.
type KillInfo struct {
	TimeMin        float64  `json:"time_min"`
	Killer         string   `json:"killer"`
	KillerID       int      `json:"killer_id"`
	Victim         string   `json:"victim"`
	VictimID       int      `json:"victim_id"`
	Assists        []string `json:"assists"`
	Bounty         int      `json:"bounty"`
	ShutdownBounty int      `json:"shutdown_bounty"`
	KillStreak     int      `json:"kill_streak"`
}

// DeathInfo represents info about a death event.
type DeathInfo struct {
	TimeMin  float64 `json:"time_min"`
	Player   string  `json:"player"`
	Position string  `json:"position"`
	Killer   string  `json:"killer"`
}

// ObjectiveKill represents an objective kill event.
type ObjectiveKill struct {
	TimeMin       float64 `json:"time_min"`
	MonsterType   string  `json:"monster_type"`
	MonsterSubType string `json:"monster_subtype"`
	Killer        string  `json:"killer"`
	KillerTeam    int     `json:"killer_team"`
}

// GoldDiff represents gold difference at 10 minutes.
type GoldDiff struct {
	Gold         int    `json:"gold"`
	OpponentGold int    `json:"opponent_gold"`
	Diff         int    `json:"diff"`
	Position     string `json:"position"`
}
