package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/zoebot/internal/embeds"
)

// handleCounter handles the /counter command.
func (b *Bot) handleCounter(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	champion := options[0].StringValue()
	var lane string
	if len(options) > 1 {
		lane = options[1].StringValue()
	}

	// Defer interaction (loading state)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Call scraper
	counters, err := b.scraperClient.GetCounters(champion, lane)
	if err != nil {
		embed := embeds.Error("Không tìm thấy dữ liệu khắc chế! Hãy kiểm tra lại tên tướng.", fmt.Sprintf("Lỗi: %v", err))
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	if len(counters) == 0 {
		embed := embeds.Error(fmt.Sprintf("Không có dữ liệu cho **%s** %s.", champion, lane), "")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	// Build Embed with cleaner format
	champDisplay := strings.Title(strings.ToLower(champion))
	title := fmt.Sprintf("⚔️ Khắc chế %s", champDisplay)

	// Determine lane display
	laneDisplay := ""
	if lane != "" {
		laneDisplay = strings.Title(strings.ToLower(lane))
	} else if len(counters) > 0 && counters[0].Lane != "" && counters[0].Lane != "All" {
		laneDisplay = counters[0].Lane
	}

	embed := &discordgo.MessageEmbed{
		Title: title,
		Color: 0xE74C3C, // Modern red
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: embeds.GetChampionIcon(champion),
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "📊 Dữ liệu từ CounterStats.net",
		},
	}

	// Build description with formatted list
	var sb strings.Builder
	if laneDisplay != "" {
		sb.WriteString(fmt.Sprintf("**Lane:** %s\n\n", laneDisplay))
	}

	// Create a clean table format in description
	for k, c := range counters {
		if k >= 5 {
			break
		}
		// Format: 🥇 Anivia — 56% WR
		medal := getMedal(k)
		sb.WriteString(fmt.Sprintf("%s **%s** — `%s`\n", medal, c.ChampionName, c.WinRate))
	}

	embed.Description = sb.String()

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

func getMedal(index int) string {
	switch index {
	case 0:
		return "🥇"
	case 1:
		return "🥈"
	case 2:
		return "🥉"
	case 3:
		return "`4.`"
	case 4:
		return "`5.`"
	default:
		return "•"
	}
}
