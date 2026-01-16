package bot

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/zoebot/internal/embeds"
	"github.com/zoebot/internal/services/riot"
)

// handleLeaderboard handles the /leaderboard command.
func (b *Bot) handleLeaderboard(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Defer response (loading state)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Get tracked players in this channel
	players := b.trackedPlayers.GetByChannel(i.ChannelID)

	if len(players) == 0 {
		embed := embeds.Warning("Chưa có người chơi nào được theo dõi trong kênh này.", "Sử dụng `/track` để thêm người chơi.")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	// Debug: log tracked players
	for _, p := range players {
		log.Printf("Tracked player: Name=%s, PUUID=%s, ChannelID=%s", p.Name, p.PUUID, p.ChannelID)
	}

	// Fetch rank info for all players concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	var rankInfos []*riot.PlayerRankInfo

	for _, player := range players {
		// Skip if PUUID is empty
		if player.PUUID == "" {
			log.Printf("Skipping player %s: empty PUUID", player.Name)
			continue
		}

		wg.Add(1)
		go func(puuid, name string) {
			defer wg.Done()

			info, err := b.riotClient.GetPlayerRankInfo(puuid, name)
			if err != nil {
				log.Printf("Failed to get rank info for %s: %v", name, err)
				return // Skip failed players
			}

			mu.Lock()
			rankInfos = append(rankInfos, info)
			mu.Unlock()
		}(player.PUUID, player.Name)
	}

	wg.Wait()

	if len(rankInfos) == 0 {
		embed := embeds.Error("Không thể lấy thông tin xếp hạng. Vui lòng thử lại sau.", "")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	// Sort by TierValue (highest first)
	sort.Slice(rankInfos, func(i, j int) bool {
		return rankInfos[i].TierValue > rankInfos[j].TierValue
	})

	// Get channel name
	channelName := "this channel"
	if channel, err := s.Channel(i.ChannelID); err == nil {
		channelName = "#" + channel.Name
	}

	// Build embed
	embed := buildLeaderboardEmbed(rankInfos, channelName)

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// buildLeaderboardEmbed creates the leaderboard embed.
func buildLeaderboardEmbed(players []*riot.PlayerRankInfo, channelName string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: "🏆 BẢNG XẾP HẠNG",
		Color: 0xF1C40F, // Gold color
	}

	if len(players) == 0 {
		embed.Description = "Không có dữ liệu"
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: "📊 Cache 10 phút",
		}
		return embed
	}

	var sb strings.Builder

	// Track queue types for footer
	queueTypes := make(map[string]bool)

	for idx, p := range players {
		if idx >= 10 { // Max 10 players
			break
		}

		// Medal/number
		medal := getLeaderboardMedal(idx)

		// Rank display
		rankStr := formatRank(p.Tier, p.Rank, p.LP)

		// Queue type indicator
		queueIcon := ""
		if p.QueueType == "RANKED_FLEX_SR" {
			queueIcon = " 👥"
			queueTypes["flex"] = true
		} else if p.QueueType == "RANKED_SOLO_5x5" {
			queueTypes["solo"] = true
		}

		// Winrate
		wrStr := ""
		if p.TotalGames > 0 {
			wrStr = fmt.Sprintf("%.1f%%", p.WinRate)
		} else {
			wrStr = "-"
		}

		// Hot streak indicator
		streakIcon := ""
		if p.HotStreak {
			streakIcon = " 🔥"
		}

		// Format: 🥇 **Player#Tag**
		// ┗ Diamond II 75LP • 58.2% (142G) 🔥 👥
		sb.WriteString(fmt.Sprintf("%s **%s**\n", medal, p.Name))
		sb.WriteString(fmt.Sprintf("┗ %s • `%s` (%dG)%s%s\n\n", rankStr, wrStr, p.TotalGames, streakIcon, queueIcon))
	}

	embed.Description = sb.String()

	// Build footer based on queue types found
	footerText := "📊 "
	if queueTypes["solo"] && queueTypes["flex"] {
		footerText += "Solo/Duo & Flex"
	} else if queueTypes["flex"] {
		footerText += "Ranked Flex 👥"
	} else if queueTypes["solo"] {
		footerText += "Ranked Solo/Duo"
	} else {
		footerText += "Ranked"
	}
	footerText += " • Cache 10 phút"

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: footerText,
	}

	return embed
}

// formatRank formats tier/rank/LP into display string.
func formatRank(tier, rank string, lp int) string {
	if tier == "UNRANKED" || tier == "" {
		return "Unranked"
	}

	if tier == "N/A" {
		return "Chưa xác định"
	}

	// Format tier name (capitalize first letter only)
	tierDisplay := strings.Title(strings.ToLower(tier))

	// Master+ don't have rank subdivisions
	if tier == "MASTER" || tier == "GRANDMASTER" || tier == "CHALLENGER" {
		return fmt.Sprintf("%s %dLP", tierDisplay, lp)
	}

	return fmt.Sprintf("%s %s %dLP", tierDisplay, rank, lp)
}

// getLeaderboardMedal returns medal emoji for leaderboard position.
func getLeaderboardMedal(idx int) string {
	switch idx {
	case 0:
		return "🥇"
	case 1:
		return "🥈"
	case 2:
		return "🥉"
	default:
		return fmt.Sprintf("`%d.`", idx+1)
	}
}
