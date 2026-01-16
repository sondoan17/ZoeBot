package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	gamedata "github.com/zoebot/internal/data"
	"github.com/zoebot/internal/embeds"
	"github.com/zoebot/internal/services/scraper"
)

// handleBuild handles the /build command.
func (b *Bot) handleBuild(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	champion := options[0].StringValue()
	role := options[1].StringValue()

	// Send searching status
	embed := embeds.Info(
		fmt.Sprintf("Đang tìm build cho **%s** ở vị trí **%s**...", champion, role),
		"🔍 Đang tìm kiếm...",
	)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})

	// Get build data
	buildData, err := b.scraperClient.GetBuild(champion, role)
	if err != nil {
		embed := embeds.Error(
			fmt.Sprintf("Không tìm thấy build cho **%s %s**.\n\n%s", champion, role, err.Error()),
			"",
		)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		})
		return
	}

	// Create build embed
	embed = createBuildEmbed(buildData)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// createBuildEmbed creates a Discord embed for build data.
func createBuildEmbed(data *scraper.BuildData) *discordgo.MessageEmbed {
	// Role display names (keep English, capitalize)
	roleDisplayNames := map[string]string{
		"top":     "TOP",
		"jungle":  "JUNGLE",
		"mid":     "MID",
		"adc":     "ADC",
		"support": "SUPPORT",
	}

	roleEmojis := map[string]string{
		"top":     "🛡️",
		"jungle":  "🌲",
		"mid":     "⚡",
		"adc":     "🏹",
		"support": "💚",
	}

	roleKey := strings.ToLower(data.Role)
	roleEmoji := roleEmojis[roleKey]
	if roleEmoji == "" {
		roleEmoji = "🎮"
	}
	roleName := roleDisplayNames[roleKey]
	if roleName == "" {
		roleName = strings.ToUpper(data.Role)
	}

	// Title
	title := fmt.Sprintf("⚔️ BUILD: %s %s %s",
		strings.ToUpper(data.Champion),
		roleEmoji,
		roleName,
	)

	embed := &discordgo.MessageEmbed{
		Title: title,
		Color: 0x3498DB, // Blue
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: embeds.GetChampionIcon(data.Champion),
		},
		Fields: make([]*discordgo.MessageEmbedField, 0),
	}

	// Set keystone rune as author icon if available
	if data.KeystoneID > 0 {
		if keystoneURL := gamedata.GetPerkIconURL(data.KeystoneID); keystoneURL != "" {
			// Use Author field for keystone icon (shows on the left of title)
			embed.Author = &discordgo.MessageEmbedAuthor{
				Name:    data.PrimaryRunes[0], // Keystone name
				IconURL: keystoneURL,
			}
		}
	}

	// Runes Section
	if len(data.PrimaryRunes) > 0 {
		runeValue := buildRuneDisplay(data)
		runeTitle := "🔮 NGỌC BỔ TRỢ"
		if data.RuneWinRate != "" {
			runeTitle = fmt.Sprintf("🔮 NGỌC BỔ TRỢ ─ %s Tỉ lệ thắng", data.RuneWinRate)
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   runeTitle,
			Value:  runeValue,
			Inline: false,
		})
	}

	// Items Section
	if len(data.CoreItems) > 0 || data.Boots != "" {
		itemValue := buildItemDisplay(data)
		itemTitle := "🗡️ TRANG BỊ CỐT LÕI"
		if data.ItemWinRate != "" {
			itemTitle = fmt.Sprintf("🗡️ TRANG BỊ CỐT LÕI ─ %s Tỉ lệ thắng", data.ItemWinRate)
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   itemTitle,
			Value:  itemValue,
			Inline: false,
		})
	}

	// Footer
	footerText := "📊 Dữ liệu từ OP.GG"
	if data.PatchVersion != "" {
		footerText = fmt.Sprintf("📊 OP.GG • Phiên bản %s", data.PatchVersion)
	}
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: footerText,
	}

	return embed
}

// buildRuneDisplay creates the rune display string.
func buildRuneDisplay(data *scraper.BuildData) string {
	var lines []string

	// Primary Tree
	if data.PrimaryTree != "" {
		lines = append(lines, fmt.Sprintf("**%s** (Chính)", data.PrimaryTree))
	}

	// Primary Runes (Keystone + 3 minor)
	if len(data.PrimaryRunes) > 0 {
		// Keystone
		lines = append(lines, fmt.Sprintf("┌ **%s**", data.PrimaryRunes[0]))

		// Minor runes
		if len(data.PrimaryRunes) > 1 {
			minorRunes := data.PrimaryRunes[1:]
			lines = append(lines, fmt.Sprintf("└ %s", strings.Join(minorRunes, " • ")))
		}
	}

	// Secondary Tree
	if data.SecondaryTree != "" {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("**%s** (Phụ)", data.SecondaryTree))
	}

	// Secondary Runes
	if len(data.SecondaryRunes) > 0 {
		lines = append(lines, fmt.Sprintf("└ %s", strings.Join(data.SecondaryRunes, " • ")))
	}

	// Stat Shards
	if len(data.StatShards) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("**Chỉ số:** %s", strings.Join(data.StatShards, " • ")))
	}

	if len(lines) == 0 {
		return "Không có dữ liệu ngọc"
	}

	return strings.Join(lines, "\n")
}

// buildItemDisplay creates the item display string.
func buildItemDisplay(data *scraper.BuildData) string {
	var lines []string

	// Starter Items (use + since bought together)
	if len(data.StarterItems) > 0 {
		starters := formatStarterItems(data.StarterItems, data.PatchVersion)
		lines = append(lines, fmt.Sprintf("**Khởi đầu:** %s", starters))
	}

	// Boots
	if data.Boots != "" {
		bootName := getItemName(data.Boots)
		bootURL := gamedata.GetItemIconURL(data.Boots, data.PatchVersion)
		lines = append(lines, fmt.Sprintf("**Giày:** [%s](%s)", bootName, bootURL))
	}

	// Core Items (use → since built in sequence)
	if len(data.CoreItems) > 0 {
		coreNames := formatItemIDsWithLinks(data.CoreItems, data.PatchVersion)
		lines = append(lines, fmt.Sprintf("**Cốt lõi:** %s", coreNames))
	}

	// Pick rate info
	if data.ItemPickRate != "" {
		lines = append(lines, fmt.Sprintf("_Tỉ lệ chọn: %s_", data.ItemPickRate))
	}

	if len(lines) == 0 {
		return "Không có dữ liệu trang bị"
	}

	return strings.Join(lines, "\n")
}

// formatStarterItems formats starter items with + separator (bought together).
func formatStarterItems(itemIDs []string, patchVersion string) string {
	var names []string
	for _, id := range itemIDs {
		name := getItemName(id)
		url := gamedata.GetItemIconURL(id, patchVersion)
		names = append(names, fmt.Sprintf("[%s](%s)", name, url))
	}
	return strings.Join(names, " + ")
}

// formatItemIDsWithLinks converts item IDs to display names with image links.
func formatItemIDsWithLinks(itemIDs []string, patchVersion string) string {
	var names []string
	for _, id := range itemIDs {
		name := getItemName(id)
		url := gamedata.GetItemIconURL(id, patchVersion)
		names = append(names, fmt.Sprintf("[%s](%s)", name, url))
	}
	return strings.Join(names, " → ")
}

// formatItemIDs converts item IDs to display names.
func formatItemIDs(itemIDs []string) string {
	var names []string
	for _, id := range itemIDs {
		name := getItemName(id)
		names = append(names, name)
	}
	return strings.Join(names, " → ")
}

// getItemName returns item name from ID using loaded data.
func getItemName(id string) string {
	return gamedata.GetItemName(id)
}
