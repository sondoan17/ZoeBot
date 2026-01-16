// Package embeds provides Discord embed builders for ZoeBot.
package embeds

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/zoebot/internal/services/ai"
	"github.com/zoebot/internal/services/riot"
)

// Colors for embeds
const (
	ColorWin     = 0x00FF00 // Green
	ColorLose    = 0xFF0000 // Red
	ColorInfo    = 0x3498DB // Blue
	ColorWarning = 0xFFFF00 // Yellow
)

// DDragonVersion is the Data Dragon version for assets.
var DDragonVersion = "16.1.1"

// GetChampionIcon returns the champion icon URL.
func GetChampionIcon(championName string) string {
	// Handle special champion names
	nameMapping := map[string]string{
		"Wukong":   "MonkeyKing",
		"Cho'Gath": "Chogath",
		"Vel'Koz":  "Velkoz",
		"Kha'Zix":  "Khazix",
		"Kai'Sa":   "Kaisa",
		"Bel'Veth": "Belveth",
		"K'Sante":  "KSante",
		"Rek'Sai":  "RekSai",
		"Kog'Maw":  "KogMaw",
	}

	cleanName := championName
	if mapped, ok := nameMapping[championName]; ok {
		cleanName = mapped
	} else {
		cleanName = strings.ReplaceAll(cleanName, " ", "")
		cleanName = strings.ReplaceAll(cleanName, "'", "")
	}

	return fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/img/champion/%s.png", DDragonVersion, cleanName)
}

// GetPositionEmoji returns emoji for each position.
func GetPositionEmoji(position string) string {
	positionEmojis := map[string]string{
		"TOP":        "🛡️",
		"JUNGLE":     "🌲",
		"MIDDLE":     "⚡",
		"BOTTOM":     "🏹",
		"UTILITY":    "💚",
		"Đường trên": "🛡️",
		"Đi rừng":    "🌲",
		"Đường giữa": "⚡",
		"Xạ thủ":     "🏹",
		"Hỗ trợ":     "💚",
	}

	if emoji, ok := positionEmojis[position]; ok {
		return emoji
	}
	return "🎮"
}

// Success creates a success embed.
func Success(message, title string) *discordgo.MessageEmbed {
	if title == "" {
		title = "✅ Thành công"
	}
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: message,
		Color:       ColorWin,
	}
}

// Error creates an error embed.
func Error(message, title string) *discordgo.MessageEmbed {
	if title == "" {
		title = "❌ Lỗi"
	}
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: message,
		Color:       ColorLose,
	}
}

// Warning creates a warning embed.
func Warning(message, title string) *discordgo.MessageEmbed {
	if title == "" {
		title = "⚠️ Cảnh báo"
	}
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: message,
		Color:       ColorWarning,
	}
}

// Info creates an info embed.
func Info(message, title string) *discordgo.MessageEmbed {
	if title == "" {
		title = "ℹ️ Thông tin"
	}
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: message,
		Color:       ColorInfo,
	}
}

// Searching creates a searching status embed.
func Searching(riotID string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "🔍 Đang tìm kiếm...",
		Description: fmt.Sprintf("Đang tìm kiếm **%s**...", riotID),
		Color:       ColorInfo,
	}
}

// Analyzing creates an analyzing status embed.
func Analyzing(riotID, matchID string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "⏳ Đang phân tích...",
		Description: fmt.Sprintf("Đang phân tích trận đấu `%s` của **%s**...", matchID, riotID),
		Color:       ColorInfo,
	}
}

// TrackingList creates an embed for tracked players list.
func TrackingList(players []string, channelName string) *discordgo.MessageEmbed {
	if len(players) == 0 {
		return &discordgo.MessageEmbed{
			Title:       "📋 Danh sách theo dõi",
			Description: "Chưa theo dõi người chơi nào trong kênh này.\nDùng `/track` để bắt đầu.",
			Color:       ColorInfo,
		}
	}

	var playerList strings.Builder
	for _, name := range players {
		playerList.WriteString(fmt.Sprintf("• **%s**\n", name))
	}

	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📋 Đang theo dõi (%d người)", len(players)),
		Description: playerList.String(),
		Color:       ColorInfo,
	}
}

// CompactAnalysis creates a compact embed with all players.
func CompactAnalysis(players []ai.PlayerAnalysis, matchData *riot.ParsedMatchData) *discordgo.MessageEmbed {
	color := ColorLose
	winText := "💀 **THUA**"
	if matchData.Win {
		color = ColorWin
		winText = "🏆 **THẮNG**"
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📊 PHÂN TÍCH TRẬN ĐẤU",
		Description: fmt.Sprintf("%s | ⏱️ %.1f phút | 🎮 %s", winText, matchData.GameDurationMinutes, matchData.GameMode),
		Color:       color,
		Fields:      make([]*discordgo.MessageEmbedField, 0, len(players)),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Match ID: %s", matchData.MatchID),
		},
	}

	for _, p := range players {
		scoreEmoji := ai.GetScoreEmoji(p.Score)
		positionEmoji := GetPositionEmoji(p.PositionVN)

		// Build field value
		var lines []string
		if p.VsOpponent != "" {
			lines = append(lines, fmt.Sprintf("⚔️ %s", p.VsOpponent))
		}
		if p.Highlight != "" {
			lines = append(lines, fmt.Sprintf("💪 %s", p.Highlight))
		}
		if p.Weakness != "" {
			lines = append(lines, fmt.Sprintf("📉 %s", p.Weakness))
		}
		if p.Comment != "" {
			lines = append(lines, fmt.Sprintf("📝 _%s_", p.Comment))
		}

		fieldValue := "Không có dữ liệu"
		if len(lines) > 0 {
			fieldValue = strings.Join(lines, "\n")
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s - %s (%s %s) - **%.1f/10**", scoreEmoji, p.Champion, p.PlayerName, positionEmoji, p.PositionVN, p.Score),
			Value:  fieldValue,
			Inline: false,
		})
	}

	return embed
}

// PlayerAnalysisEmbed creates a detailed embed for a single player.
func PlayerAnalysisEmbed(p ai.PlayerAnalysis, matchData *riot.ParsedMatchData) *discordgo.MessageEmbed {
	color := ColorLose
	if matchData.Win {
		color = ColorWin
	}

	scoreEmoji := ai.GetScoreEmoji(p.Score)
	positionEmoji := GetPositionEmoji(p.PositionVN)

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s %s - %s", scoreEmoji, p.Champion, p.PlayerName),
		Description: fmt.Sprintf("%s %s | **%.1f/10**", positionEmoji, p.PositionVN, p.Score),
		Color:       color,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: GetChampionIcon(p.Champion),
		},
		Fields: make([]*discordgo.MessageEmbedField, 0),
	}

	if p.VsOpponent != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "⚔️ So sánh với đối thủ",
			Value:  p.VsOpponent,
			Inline: false,
		})
	}

	if p.RoleAnalysis != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🎭 Vai trò",
			Value:  p.RoleAnalysis,
			Inline: true,
		})
	}

	if p.Highlight != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "💪 Điểm mạnh",
			Value:  p.Highlight,
			Inline: true,
		})
	}

	if p.Weakness != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "📉 Điểm yếu",
			Value:  p.Weakness,
			Inline: false,
		})
	}

	if p.Comment != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "📝 Nhận xét",
			Value:  fmt.Sprintf("_%s_", p.Comment),
			Inline: false,
		})
	}

	if p.TimelineAnalysis != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "⏱️ Timeline",
			Value:  p.TimelineAnalysis,
			Inline: false,
		})
	}

	return embed
}

// NewMatchNotification creates an embed for new match notification.
func NewMatchNotification(playerNames []string) *discordgo.MessageEmbed {
	mention := strings.Join(playerNames, ", ")
	return &discordgo.MessageEmbed{
		Title:       "🚨 TRẬN MỚI",
		Description: fmt.Sprintf("%s vừa chơi xong trận!\n⏳ Đang phân tích...", mention),
		Color:       ColorInfo,
	}
}
