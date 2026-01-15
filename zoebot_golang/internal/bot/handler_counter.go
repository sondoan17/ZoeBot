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
	// Note: normalize is done inside scraper client, but we can do it here if needed.
	// Scraper handles normalization.
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

	// Build Embed
	title := fmt.Sprintf("⚔️ Kèo khắc chế: %s", strings.Title(strings.ToLower(champion)))
	if lane != "" {
		title += fmt.Sprintf(" (%s)", strings.Title(strings.ToLower(lane)))
	}

	description := fmt.Sprintf("Dưới đây là Top 5 tướng khắc chế **%s** mạnh nhất (theo Winrate).", strings.Title(strings.ToLower(champion)))

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       0xFF0000, // Red for danger/counter
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: embeds.GetChampionIcon(champion),
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Dữ liệu từ U.GG • Tự động cập nhật",
		},
	}

	// Create table-like content using fields
	// Name | Winrate | Matches
	var names, wrs, matches []string

	for k, c := range counters {
		if k >= 5 { break }
		// c.ChampionName might be "Kennen"
		names = append(names, fmt.Sprintf("**%d. %s**", k+1, c.ChampionName))
		wrs = append(wrs, fmt.Sprintf("`%s`", c.WinRate))
		matches = append(matches, fmt.Sprintf("%s trận", c.Matches))
	}

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "Tướng",
			Value:  strings.Join(names, "\n"),
			Inline: true,
		},
		{
			Name:   "Tỉ lệ thắng",
			Value:  strings.Join(wrs, "\n"),
			Inline: true,
		},
		{
			Name:   "Số trận",
			Value:  strings.Join(matches, "\n"),
			Inline: true,
		},
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}
