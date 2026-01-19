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
	data, err := b.scraperClient.GetCounters(champion, lane)
	if err != nil {
		embed := embeds.Error("Không tìm thấy dữ liệu khắc chế! Hãy kiểm tra lại tên tướng.", fmt.Sprintf("Lỗi: %v", err))
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	if len(data.BestPicks) == 0 && len(data.WorstPicks) == 0 {
		embed := embeds.Error(fmt.Sprintf("Không có dữ liệu cho **%s** %s.", champion, lane), "")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	// Build Embed
	champDisplay := strings.Title(strings.ToLower(champion))
	title := fmt.Sprintf("⚔️ Matchup: %s", champDisplay)
	if data.Lane != "" {
		title += fmt.Sprintf(" (%s)", data.Lane)
	}

	embed := &discordgo.MessageEmbed{
		Title: title,
		Color: 0xE74C3C,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: embeds.GetChampionIcon(champion),
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "📊 CounterStats.net",
		},
	}

	// Build Best Picks column (counters the target)
	var bestPicksStr strings.Builder
	for k, c := range data.BestPicks {
		if k >= 5 {
			break
		}
		bestPicksStr.WriteString(fmt.Sprintf("%s %s `%s`\n", getMedal(k), c.ChampionName, c.WinRate))
	}

	// Build Worst Picks column (weak against target)
	var worstPicksStr strings.Builder
	for k, c := range data.WorstPicks {
		if k >= 5 {
			break
		}
		worstPicksStr.WriteString(fmt.Sprintf("%s %s `%s`\n", getMedal(k), c.ChampionName, c.WinRate))
	}

	embed.Fields = []*discordgo.MessageEmbedField{}

	if bestPicksStr.Len() > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "✅ Khắc chế " + champDisplay,
			Value:  bestPicksStr.String(),
			Inline: true,
		})
	}

	if worstPicksStr.Len() > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "❌ Bị " + champDisplay + " khắc chế",
			Value:  worstPicksStr.String(),
			Inline: true,
		})
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})

	// Save context for AI chat replies
	if msg, err := s.InteractionResponse(i.Interaction); err == nil {
		// Convert counter data to simple format for context
		bestPicks := make([]map[string]string, 0)
		for _, c := range data.BestPicks {
			bestPicks = append(bestPicks, map[string]string{
				"champion": c.ChampionName,
				"winrate":  c.WinRate,
			})
		}
		worstPicks := make([]map[string]string, 0)
		for _, c := range data.WorstPicks {
			worstPicks = append(worstPicks, map[string]string{
				"champion": c.ChampionName,
				"winrate":  c.WinRate,
			})
		}

		contextData := map[string]interface{}{
			"target_champion": champion,
			"lane":            data.Lane,
			"best_picks":      bestPicks,
			"worst_picks":     worstPicks,
		}
		b.saveMessageContext(msg.ID, "counter", contextData)
	}
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
