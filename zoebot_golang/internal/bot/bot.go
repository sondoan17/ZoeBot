// Package bot provides the Discord bot core for ZoeBot.
package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/zoebot/internal/config"
	"github.com/zoebot/internal/embeds"
	"github.com/zoebot/internal/services/ai"
	"github.com/zoebot/internal/services/riot"
	"github.com/zoebot/internal/services/scraper"
	"github.com/zoebot/internal/storage"
)

// AnalysisCache stores analysis results for button interactions.
type AnalysisCache struct {
	Players   []ai.PlayerAnalysis
	MatchData *riot.ParsedMatchData
}

// MessageContext stores context for AI chat replies.
type MessageContext struct {
	Type      string                 // "analysis", "build", "counter"
	Data      map[string]interface{} // Context data
	CreatedAt time.Time
}

// Bot represents the Discord bot.
type Bot struct {
	session         *discordgo.Session
	cfg             *config.Config
	riotClient      *riot.Client
	aiClient        *ai.Client
	scraperClient   *scraper.Client
	trackedPlayers  *storage.TrackedPlayersStore
	analyzedMatches map[string][]string        // matchID -> []channelID
	analysisCache   map[string]*AnalysisCache  // matchID -> analysis result
	messageContext  map[string]*MessageContext // messageID -> context for AI chat
	analyzesMu      sync.RWMutex
	cacheMu         sync.RWMutex
	contextMu       sync.RWMutex
	stopPolling     chan struct{}
	commands        []*discordgo.ApplicationCommand
}

// New creates a new Bot instance.
func New(cfg *config.Config) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsMessageContent

	// Create Redis client and tracked players store
	redisClient := storage.NewRedisClient(cfg)
	trackedPlayers := storage.NewTrackedPlayersStore(redisClient, cfg.RedisKeyTrackedPlayers)

	// Load tracked players
	if err := trackedPlayers.Load(); err != nil {
		log.Printf("Load players failed: %v", err)
	}

	bot := &Bot{
		session:         session,
		cfg:             cfg,
		riotClient:      riot.NewClient(cfg, redisClient),
		aiClient:        ai.NewClient(cfg),
		scraperClient:   scraper.NewClient(redisClient),
		trackedPlayers:  trackedPlayers,
		analyzedMatches: make(map[string][]string),
		analysisCache:   make(map[string]*AnalysisCache),
		messageContext:  make(map[string]*MessageContext),
		stopPolling:     make(chan struct{}),
	}

	// Register handlers
	session.AddHandler(bot.onReady)
	session.AddHandler(bot.onInteractionCreate)
	session.AddHandler(bot.onMessageCreate)

	return bot, nil
}

// Start connects to Discord and starts the bot.
func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord session: %w", err)
	}

	log.Println("Connected to Discord")

	// Register slash commands
	if err := b.registerCommands(); err != nil {
		log.Printf("Register commands failed: %v", err)
	}

	// Start polling task
	go b.pollMatches()

	// Start context cleanup task
	go b.startContextCleanup()

	return nil
}

// Stop gracefully shuts down the bot.
func (b *Bot) Stop() error {
	close(b.stopPolling)
	b.trackedPlayers.Save()
	return b.session.Close()
}

// onReady is called when the bot is ready.
func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Bot ready: %s", event.User.Username)
}

// registerCommands registers all slash commands.
func (b *Bot) registerCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Kiểm tra bot còn sống không",
		},
		{
			Name:        "track",
			Description: "Theo dõi người chơi - thông báo khi có trận mới",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "riot_id",
					Description: "Tên người chơi (VD: Faker#KR1)",
					Required:    true,
				},
			},
		},
		{
			Name:        "untrack",
			Description: "Huỷ theo dõi người chơi",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "riot_id",
					Description: "Tên người chơi cần huỷ theo dõi",
					Required:    true,
				},
			},
		},
		{
			Name:        "list",
			Description: "Xem danh sách người chơi đang theo dõi",
		},
		{
			Name:        "analyze",
			Description: "Phân tích trận đấu gần nhất của người chơi",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "riot_id",
					Description: "Tên người chơi (VD: Faker#KR1)",
					Required:    true,
				},
			},
		},
		{
			Name:        "counter",
			Description: "Tìm tướng khắc chế (Winrate & Tips)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "champion",
					Description: "Tên tướng cần khắc chế (VD: Yasuo)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "lane",
					Description: "Đường/Vị trí (top, jungle, mid, adc, support)",
					Required:    false,
				},
			},
		},
		{
			Name:        "leaderboard",
			Description: "Xem bảng xếp hạng người chơi đang theo dõi",
		},
		{
			Name:        "build",
			Description: "Xem build tướng (runes, items) từ OP.GG",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "champion",
					Description: "Tên tướng (VD: Yasuo, Lee Sin)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "role",
					Description: "Vị trí (top, jungle, mid, adc, support)",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Top", Value: "top"},
						{Name: "Jungle", Value: "jungle"},
						{Name: "Mid", Value: "mid"},
						{Name: "ADC", Value: "adc"},
						{Name: "Support", Value: "support"},
					},
				},
			},
		},
	}

	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, cmd := range commands {
		registered, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, "", cmd)
		if err != nil {
			log.Printf("Command %s failed: %v", cmd.Name, err)
			continue
		}
		registeredCommands[i] = registered
	}

	b.commands = registeredCommands
	log.Printf("Registered %d commands", len(registeredCommands))
	return nil
}

// onInteractionCreate handles slash command interactions.
func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionApplicationCommand {
		switch i.ApplicationCommandData().Name {
		case "ping":
			b.handlePing(s, i)
		case "track":
			b.handleTrack(s, i)
		case "untrack":
			b.handleUntrack(s, i)
		case "list":
			b.handleList(s, i)
		case "analyze":
			b.handleAnalyze(s, i)
		case "counter":
			b.handleCounter(s, i)
		case "leaderboard":
			b.handleLeaderboard(s, i)
		case "build":
			b.handleBuild(s, i)
		}
	} else if i.Type == discordgo.InteractionMessageComponent {
		b.handleComponentInteraction(s, i)
	}
}

// handlePing handles the /ping command.
func (b *Bot) handlePing(s *discordgo.Session, i *discordgo.InteractionCreate) {
	latency := s.HeartbeatLatency().Milliseconds()
	embed := embeds.Success(
		fmt.Sprintf("🏓 Pong! Độ trễ: **%dms**", latency),
		"✅ Bot đang hoạt động",
	)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

// handleTrack handles the /track command.
func (b *Bot) handleTrack(s *discordgo.Session, i *discordgo.InteractionCreate) {
	riotID := i.ApplicationCommandData().Options[0].StringValue()

	// Validate format
	if !strings.Contains(riotID, "#") {
		embed := embeds.Error("Sai định dạng! Vui lòng dùng: `Name#Tag` (VD: Faker#KR1)", "")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Send searching status
	embed := embeds.Searching(riotID)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})

	parts := strings.SplitN(riotID, "#", 2)
	gameName, tagLine := parts[0], parts[1]

	// Get PUUID
	puuid, err := b.riotClient.GetPUUIDByRiotID(gameName, tagLine)
	if err != nil || puuid == "" {
		embed := embeds.Error(fmt.Sprintf("Không tìm thấy người chơi **%s**. Kiểm tra lại tên và tag.", riotID), "")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	// Check if already tracking
	if existing, ok := b.trackedPlayers.Get(puuid); ok {
		if existing.ChannelID == i.ChannelID {
			embed := embeds.Warning(fmt.Sprintf("**%s** đã được theo dõi trong kênh này rồi.", riotID), "")
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Embeds: &[]*discordgo.MessageEmbed{embed},
			})
			return
		}
	}

	// Get latest match to initialize
	matches, _ := b.riotClient.GetMatchIDsByPUUID(puuid, 1)
	var lastMatchID string
	if len(matches) > 0 {
		lastMatchID = matches[0]
	}

	// Add to tracking
	b.trackedPlayers.Set(puuid, &storage.TrackedPlayer{
		PUUID:       puuid,
		LastMatchID: lastMatchID,
		ChannelID:   i.ChannelID,
		Name:        riotID,
	})
	b.trackedPlayers.Save()

	embed = embeds.Success(
		fmt.Sprintf("Đã thêm **%s** vào danh sách theo dõi!\nBot sẽ thông báo khi có trận mới.", riotID),
		"✅ Đã theo dõi",
	)
	embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
		URL: embeds.GetChampionIcon("Zoe"),
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})

	log.Printf("Tracked: %s", riotID)
}

// handleUntrack handles the /untrack command.
func (b *Bot) handleUntrack(s *discordgo.Session, i *discordgo.InteractionCreate) {
	riotID := i.ApplicationCommandData().Options[0].StringValue()

	// Validate format
	if !strings.Contains(riotID, "#") {
		embed := embeds.Error("Sai định dạng! Vui lòng dùng: `Name#Tag` (VD: Faker#KR1)", "")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	parts := strings.SplitN(riotID, "#", 2)
	gameName, tagLine := parts[0], parts[1]

	// Get PUUID
	puuid, err := b.riotClient.GetPUUIDByRiotID(gameName, tagLine)
	if err != nil || puuid == "" {
		embed := embeds.Error(fmt.Sprintf("Không tìm thấy **%s** trong danh sách đang theo dõi.", riotID), "")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
		return
	}

	if _, ok := b.trackedPlayers.Get(puuid); ok {
		b.trackedPlayers.Delete(puuid)
		b.trackedPlayers.Save()

		embed := embeds.Success(fmt.Sprintf("Đã huỷ theo dõi **%s**.", riotID), "")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
		log.Printf("Untracked: %s", riotID)
	} else {
		embed := embeds.Error(fmt.Sprintf("Không tìm thấy **%s** trong danh sách đang theo dõi.", riotID), "")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	}
}

// handleList handles the /list command.
func (b *Bot) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelPlayers := b.trackedPlayers.GetByChannel(i.ChannelID)

	var playerNames []string
	for _, p := range channelPlayers {
		playerNames = append(playerNames, p.Name)
	}

	channelName := "Unknown"
	if channel, err := s.Channel(i.ChannelID); err == nil {
		channelName = channel.Name
	}

	embed := embeds.TrackingList(playerNames, channelName)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

// handleAnalyze handles the /analyze command.
func (b *Bot) handleAnalyze(s *discordgo.Session, i *discordgo.InteractionCreate) {
	riotID := i.ApplicationCommandData().Options[0].StringValue()

	// Validate format
	if !strings.Contains(riotID, "#") {
		embed := embeds.Error("Sai định dạng! Vui lòng dùng: `Name#Tag` (VD: Faker#KR1)", "")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Send searching status
	embed := embeds.Searching(riotID)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})

	parts := strings.SplitN(riotID, "#", 2)
	gameName, tagLine := parts[0], parts[1]

	// Get PUUID
	puuid, err := b.riotClient.GetPUUIDByRiotID(gameName, tagLine)
	if err != nil || puuid == "" {
		embed := embeds.Error(fmt.Sprintf("Không tìm thấy người chơi **%s**. Kiểm tra lại tên và tag.", riotID), "")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	// Get latest match
	matches, err := b.riotClient.GetMatchIDsByPUUID(puuid, 1)
	if err != nil || len(matches) == 0 {
		embed := embeds.Error("Người chơi này chưa đánh trận nào gần đây.", "")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	matchID := matches[0]

	// Update status
	embed = embeds.Analyzing(riotID, matchID)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})

	// Get match details and timeline
	matchDetails, err := b.riotClient.GetMatchDetails(matchID)
	if err != nil {
		embed := embeds.Error("Không thể lấy dữ liệu chi tiết của trận đấu.", "")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	timeline, _ := b.riotClient.GetMatchTimeline(matchID)

	// Parse match data
	matchData := b.riotClient.ParseMatchData(matchDetails, puuid, timeline)
	if matchData == nil {
		embed := embeds.Error("Không thể xử lý dữ liệu trận đấu.", "")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	// Get AI analysis
	analysisResult, err := b.aiClient.AnalyzeMatch(matchData)
	if err != nil {
		embed := embeds.Error(fmt.Sprintf("Lỗi AI: %s", err.Error()[:min(200, len(err.Error()))]), "")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	// Cache analysis result for button interactions
	b.cacheAnalysis(matchID, analysisResult.Players, matchData)

	// Create embed
	embed = embeds.CompactAnalysis(analysisResult.Players, matchData)

	// Create buttons
	buttons := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    "👤 Xem chi tiết từng người",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("detail_%s_%s", matchID, puuid),
		},
		discordgo.Button{
			Label:    "🔗 Copy Match ID",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("copy_%s", matchID),
		},
	}

	// Check if not tracked and add track button
	if _, ok := b.trackedPlayers.Get(puuid); !ok {
		buttons = append(buttons, discordgo.Button{
			Label:    "📌 Track người chơi này",
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("track_%s_%s", riotID, i.ChannelID),
		})
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: buttons},
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &components,
	})

	// Save context for AI chat replies
	if msg, err := s.InteractionResponse(i.Interaction); err == nil {
		contextData := map[string]interface{}{
			"match_id":      matchID,
			"target":        riotID,
			"win":           matchData.Win,
			"game_mode":     matchData.GameMode,
			"duration":      matchData.GameDurationMinutes,
			"players":       analysisResult.Players,
			"lane_matchups": matchData.LaneMatchups,
		}
		b.saveMessageContext(msg.ID, "analysis", contextData)
	}
}

// handleComponentInteraction handles button/component interactions.
func (b *Bot) handleComponentInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	switch {
	case strings.HasPrefix(customID, "detail_"), strings.HasPrefix(customID, "full_"):
		b.handleDetailButton(s, i, customID)

	case strings.HasPrefix(customID, "copy_"):
		matchID := strings.TrimPrefix(customID, "copy_")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("```\n%s\n```", matchID),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

	case strings.HasPrefix(customID, "track_"):
		parts := strings.SplitN(strings.TrimPrefix(customID, "track_"), "_", 2)
		if len(parts) < 2 {
			return
		}
		riotID := parts[0]
		channelID := parts[1]

		riotParts := strings.SplitN(riotID, "#", 2)
		if len(riotParts) < 2 {
			return
		}

		puuid, err := b.riotClient.GetPUUIDByRiotID(riotParts[0], riotParts[1])
		if err != nil || puuid == "" {
			return
		}

		matches, _ := b.riotClient.GetMatchIDsByPUUID(puuid, 1)
		var lastMatchID string
		if len(matches) > 0 {
			lastMatchID = matches[0]
		}

		b.trackedPlayers.Set(puuid, &storage.TrackedPlayer{
			PUUID:       puuid,
			LastMatchID: lastMatchID,
			ChannelID:   channelID,
			Name:        riotID,
		})
		b.trackedPlayers.Save()

		embed := embeds.Success(fmt.Sprintf("Đã thêm **%s** vào danh sách theo dõi!", riotID), "")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
	}
}

// handleDetailButton handles detail/full analysis button clicks.
func (b *Bot) handleDetailButton(s *discordgo.Session, i *discordgo.InteractionCreate, customID string) {
	// Parse customID: detail_matchID_puuid or full_matchID_puuid
	var remainder string
	if strings.HasPrefix(customID, "detail_") {
		remainder = strings.TrimPrefix(customID, "detail_")
	} else {
		remainder = strings.TrimPrefix(customID, "full_")
	}

	// Format: VN2_123456789_puuid
	// matchID is "VN2_123456789", puuid is after the last underscore
	// Find the last underscore to separate matchID from puuid
	lastUnderscoreIdx := strings.LastIndex(remainder, "_")
	var matchID string
	if lastUnderscoreIdx > 0 {
		matchID = remainder[:lastUnderscoreIdx]
	} else {
		matchID = remainder
	}

	// Get cached analysis
	cache := b.getAnalysisCache(matchID)
	if cache == nil {
		embed := embeds.Error("Dữ liệu phân tích đã hết hạn. Vui lòng dùng `/analyze` để phân tích lại.", "")
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Printf("Error responding to interaction: %v", err)
		}
		return
	}

	// Create embeds for each player
	var playerEmbeds []*discordgo.MessageEmbed
	for _, p := range cache.Players {
		playerEmbeds = append(playerEmbeds, embeds.PlayerAnalysisEmbed(p, cache.MatchData))
	}

	// Discord allows max 10 embeds per message, split if needed
	if len(playerEmbeds) > 10 {
		playerEmbeds = playerEmbeds[:10]
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: playerEmbeds,
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Error responding with player embeds: %v", err)
	}
}

// pollMatches runs the background task to check for new matches.
func (b *Bot) pollMatches() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Println("Polling started (1min interval)")

	for {
		select {
		case <-b.stopPolling:
			log.Println("Polling stopped")
			return
		case <-ticker.C:
			b.checkMatches()
		}
	}
}

// checkMatches checks for new matches for all tracked players.
// Optimized: Parallel processing with semaphore-based rate limiting
func (b *Bot) checkMatches() {
	players := b.trackedPlayers.GetAll()
	if len(players) == 0 {
		return
	}

	var wg sync.WaitGroup
	// Semaphore to limit concurrent API requests (Riot rate limit friendly)
	sem := make(chan struct{}, 5) // Max 5 concurrent requests
	// Rate limiter: 1 request per 100ms = 10 req/sec (well under Riot's 20/sec)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()

	for puuid, data := range players {
		select {
		case <-ctx.Done():
			log.Println("Polling timeout, waiting for remaining goroutines...")
			wg.Wait()
			return
		case <-ticker.C:
			// Rate limit: wait for next tick
		}

		// Copy data to avoid data race (data is already a copy from GetAll)
		playerCopy := data

		wg.Add(1)
		go func(p string, d *storage.TrackedPlayer) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			b.checkPlayerMatch(p, d)
		}(puuid, &playerCopy)
	}

	wg.Wait()
}

// checkPlayerMatch checks for new match for a single player.
func (b *Bot) checkPlayerMatch(puuid string, data *storage.TrackedPlayer) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic checking %s: %v", data.Name, r)
		}
	}()

	matches, err := b.riotClient.GetMatchIDsByPUUID(puuid, 1)
	if err != nil || len(matches) == 0 {
		return
	}

	latestMatchID := matches[0]
	oldMatchID := data.LastMatchID

	// No new match
	if latestMatchID == oldMatchID {
		return
	}

	// Update last match ID
	b.trackedPlayers.UpdateLastMatch(puuid, latestMatchID)
	b.trackedPlayers.Save()

	// First time tracking, just initialize
	if oldMatchID == "" {
		return
	}

	log.Printf("New match: %s", data.Name)

	// Check if already analyzed for this channel
	b.analyzesMu.RLock()
	channels, exists := b.analyzedMatches[latestMatchID]
	b.analyzesMu.RUnlock()

	if exists {
		for _, ch := range channels {
			if ch == data.ChannelID {
				return
			}
		}
	}

	// Find all tracked players in this match for this channel
	allPlayers := b.trackedPlayers.GetAll()
	var playersInMatch []string
	for _, p := range allPlayers {
		if p.ChannelID == data.ChannelID && p.LastMatchID == latestMatchID {
			playersInMatch = append(playersInMatch, fmt.Sprintf("**%s**", p.Name))
		}
	}

	playersMention := strings.Join(playersInMatch, ", ")

	// Send notification
	embed := embeds.Info(
		fmt.Sprintf("%s vừa chơi xong trận!\n⏳ Đang phân tích...", playersMention),
		"🚨 TRẬN MỚI",
	)

	msg, err := b.session.ChannelMessageSendEmbed(data.ChannelID, embed)
	if err != nil {
		return
	}

	// Get match details and analyze
	matchDetails, err := b.riotClient.GetMatchDetails(latestMatchID)
	if err != nil {
		embed := embeds.Error("Không thể lấy dữ liệu trận đấu từ Riot API.", "")
		b.session.ChannelMessageEditEmbed(data.ChannelID, msg.ID, embed)
		return
	}

	timeline, _ := b.riotClient.GetMatchTimeline(latestMatchID)
	matchData := b.riotClient.ParseMatchData(matchDetails, puuid, timeline)

	if matchData == nil {
		embed := embeds.Error("Không thể xử lý dữ liệu trận đấu.", "")
		b.session.ChannelMessageEditEmbed(data.ChannelID, msg.ID, embed)
		return
	}

	analysisResult, err := b.aiClient.AnalyzeMatch(matchData)
	if err != nil {
		embed := embeds.Error(fmt.Sprintf("Lỗi AI: %s", err.Error()[:min(200, len(err.Error()))]), "")
		b.session.ChannelMessageEditEmbed(data.ChannelID, msg.ID, embed)
		return
	}

	// Cache analysis result for button interactions
	b.cacheAnalysis(latestMatchID, analysisResult.Players, matchData)

	// Create embed with analysis
	embed = embeds.CompactAnalysis(analysisResult.Players, matchData)

	// Create buttons
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "📊 Xem phân tích đầy đủ",
					Style:    discordgo.PrimaryButton,
					CustomID: fmt.Sprintf("full_%s_%s", latestMatchID, puuid),
				},
				discordgo.Button{
					Label:    "🔗 Copy Match ID",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("copy_%s", latestMatchID),
				},
			},
		},
	}

	// Edit the message with analysis
	b.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         msg.ID,
		Channel:    data.ChannelID,
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &components,
	})

	// Save context for AI chat replies
	contextData := map[string]interface{}{
		"match_id":      latestMatchID,
		"target":        data.Name,
		"win":           matchData.Win,
		"game_mode":     matchData.GameMode,
		"duration":      matchData.GameDurationMinutes,
		"players":       analysisResult.Players,
		"lane_matchups": matchData.LaneMatchups,
	}
	b.saveMessageContext(msg.ID, "analysis", contextData)

	// Mark as analyzed
	b.analyzesMu.Lock()
	b.analyzedMatches[latestMatchID] = append(b.analyzedMatches[latestMatchID], data.ChannelID)

	// Cleanup old entries
	if len(b.analyzedMatches) > 50 {
		for k := range b.analyzedMatches {
			delete(b.analyzedMatches, k)
			break
		}
	}
	b.analyzesMu.Unlock()

	log.Printf("Analyzed: %s", playersMention)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// cacheAnalysis stores analysis result for later button interactions.
func (b *Bot) cacheAnalysis(matchID string, players []ai.PlayerAnalysis, matchData *riot.ParsedMatchData) {
	b.cacheMu.Lock()
	defer b.cacheMu.Unlock()

	b.analysisCache[matchID] = &AnalysisCache{
		Players:   players,
		MatchData: matchData,
	}

	// Cleanup old entries (keep max 20)
	if len(b.analysisCache) > 20 {
		for k := range b.analysisCache {
			delete(b.analysisCache, k)
			break
		}
	}
}

// getAnalysisCache retrieves cached analysis result.
func (b *Bot) getAnalysisCache(matchID string) *AnalysisCache {
	b.cacheMu.RLock()
	defer b.cacheMu.RUnlock()
	return b.analysisCache[matchID]
}

// saveMessageContext saves context for a bot message (for AI chat replies).
func (b *Bot) saveMessageContext(messageID string, contextType string, data map[string]interface{}) {
	b.contextMu.Lock()
	defer b.contextMu.Unlock()

	b.messageContext[messageID] = &MessageContext{
		Type:      contextType,
		Data:      data,
		CreatedAt: time.Now(),
	}

	// Cleanup old entries (keep max 100, remove entries older than 24 hours)
	if len(b.messageContext) > 100 {
		ttlAgo := time.Now().Add(-24 * time.Hour)
		for id, ctx := range b.messageContext {
			if ctx.CreatedAt.Before(ttlAgo) {
				delete(b.messageContext, id)
			}
		}
		// If still too many, remove oldest
		if len(b.messageContext) > 100 {
			for id := range b.messageContext {
				delete(b.messageContext, id)
				break
			}
		}
	}
}

// getMessageContext retrieves context for a bot message.
func (b *Bot) getMessageContext(messageID string) *MessageContext {
	b.contextMu.RLock()
	defer b.contextMu.RUnlock()
	return b.messageContext[messageID]
}

// startContextCleanup starts background goroutine to cleanup expired contexts.
func (b *Bot) startContextCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	log.Println("Context cleanup started (10min interval)")

	for {
		select {
		case <-b.stopPolling:
			log.Println("Context cleanup stopped")
			return
		case <-ticker.C:
			b.cleanupOldContexts()
		}
	}
}

// cleanupOldContexts removes contexts older than 24 hours.
func (b *Bot) cleanupOldContexts() {
	b.contextMu.Lock()
	defer b.contextMu.Unlock()

	if len(b.messageContext) == 0 {
		return
	}

	ttlAgo := time.Now().Add(-24 * time.Hour)
	deleted := 0

	for id, ctx := range b.messageContext {
		if ctx.CreatedAt.Before(ttlAgo) {
			delete(b.messageContext, id)
			deleted++
		}
	}

	if deleted > 0 {
		log.Printf("Cleaned up %d expired contexts, %d remaining", deleted, len(b.messageContext))
	}
}

// onMessageCreate handles message create events (for reply-based AI chat).
func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if this is a reply to a bot message
	if m.MessageReference == nil {
		return
	}

	// Get the referenced message
	refMsg, err := s.ChannelMessage(m.ChannelID, m.MessageReference.MessageID)
	if err != nil {
		return
	}

	// Check if referenced message is from the bot
	if refMsg.Author.ID != s.State.User.ID {
		return
	}

	// Get context for the referenced message
	ctx := b.getMessageContext(refMsg.ID)
	if ctx == nil {
		// No context found, ignore
		return
	}

	// User's question
	question := strings.TrimSpace(m.Content)
	if question == "" {
		return
	}

	// Show typing indicator
	s.ChannelTyping(m.ChannelID)

	// Call AI with context
	response, err := b.aiClient.ChatWithContext(ctx.Type, ctx.Data, question)
	if err != nil {
		log.Printf("AI chat error: %v", err)
		embed := embeds.Error("Không thể trả lời lúc này. Thử lại sau nhé!", "")
		s.ChannelMessageSendEmbedReply(m.ChannelID, embed, m.Reference())
		return
	}

	// Send response as reply
	s.ChannelMessageSendReply(m.ChannelID, response, m.Reference())
}
