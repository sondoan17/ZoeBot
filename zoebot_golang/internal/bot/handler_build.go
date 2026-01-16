package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
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
	// Role emoji mapping
	roleEmojis := map[string]string{
		"top":     "🛡️",
		"jungle":  "🌲",
		"mid":     "⚡",
		"adc":     "🏹",
		"support": "💚",
	}

	roleEmoji := roleEmojis[strings.ToLower(data.Role)]
	if roleEmoji == "" {
		roleEmoji = "🎮"
	}

	// Title
	title := fmt.Sprintf("⚔️ BUILD: %s %s %s",
		strings.ToUpper(data.Champion),
		roleEmoji,
		strings.ToUpper(data.Role),
	)

	embed := &discordgo.MessageEmbed{
		Title: title,
		Color: 0x3498DB, // Blue
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: embeds.GetChampionIcon(data.Champion),
		},
		Fields: make([]*discordgo.MessageEmbedField, 0),
	}

	// Runes Section
	if len(data.PrimaryRunes) > 0 {
		runeValue := buildRuneDisplay(data)
		runeTitle := "🔮 RUNES"
		if data.RuneWinRate != "" {
			runeTitle = fmt.Sprintf("🔮 RUNES ─ %s WR", data.RuneWinRate)
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
		itemTitle := "🗡️ CORE BUILD"
		if data.ItemWinRate != "" {
			itemTitle = fmt.Sprintf("🗡️ CORE BUILD ─ %s WR", data.ItemWinRate)
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   itemTitle,
			Value:  itemValue,
			Inline: false,
		})
	}

	// Footer
	footerText := "📊 Data from OP.GG"
	if data.PatchVersion != "" {
		footerText = fmt.Sprintf("📊 OP.GG • Patch %s", data.PatchVersion)
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
		lines = append(lines, fmt.Sprintf("**%s** (Primary)", data.PrimaryTree))
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
		lines = append(lines, fmt.Sprintf("**%s** (Secondary)", data.SecondaryTree))
	}

	// Secondary Runes
	if len(data.SecondaryRunes) > 0 {
		lines = append(lines, fmt.Sprintf("└ %s", strings.Join(data.SecondaryRunes, " • ")))
	}

	// Stat Shards
	if len(data.StatShards) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("**Stats:** %s", strings.Join(data.StatShards, " • ")))
	}

	if len(lines) == 0 {
		return "Không có dữ liệu rune"
	}

	return strings.Join(lines, "\n")
}

// buildItemDisplay creates the item display string.
func buildItemDisplay(data *scraper.BuildData) string {
	var lines []string

	// Starter Items
	if len(data.StarterItems) > 0 {
		starters := formatItemIDs(data.StarterItems)
		lines = append(lines, fmt.Sprintf("**Start:** %s", starters))
	}

	// Boots
	if data.Boots != "" {
		bootName := getItemName(data.Boots)
		lines = append(lines, fmt.Sprintf("**Boots:** %s", bootName))
	}

	// Core Items
	if len(data.CoreItems) > 0 {
		coreNames := formatItemIDs(data.CoreItems)
		lines = append(lines, fmt.Sprintf("**Core:** %s", coreNames))
	}

	// Pick rate info
	if data.ItemPickRate != "" {
		lines = append(lines, fmt.Sprintf("_Pick Rate: %s_", data.ItemPickRate))
	}

	if len(lines) == 0 {
		return "Không có dữ liệu item"
	}

	return strings.Join(lines, "\n")
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

// getItemName returns item name from ID (basic mapping).
// For full names, would need Data Dragon API.
var itemNames = map[string]string{
	// Boots
	"3006": "Berserker's Greaves",
	"3009": "Boots of Swiftness",
	"3020": "Sorcerer's Shoes",
	"3047": "Plated Steelcaps",
	"3111": "Mercury's Treads",
	"3117": "Mobility Boots",
	"3158": "Ionian Boots",

	// Jungle Items
	"1102": "Gustwalker Hatchling",
	"1101": "Scorchclaw Pup",
	"1103": "Mosstomper Seedling",

	// Starter Items
	"2003": "Health Potion",
	"2031": "Refillable Potion",
	"2033": "Corrupting Potion",
	"1054": "Doran's Shield",
	"1055": "Doran's Blade",
	"1056": "Doran's Ring",
	"1082": "Dark Seal",
	"3850": "Spellthief's Edge",
	"3854": "Steel Shoulderguards",
	"3858": "Relic Shield",
	"3862": "Spectral Sickle",

	// Mythics / Core Items
	"6653": "Liandry's Torment",
	"6655": "Luden's Companion",
	"6656": "Everfrost",
	"6657": "Rod of Ages",
	"6662": "Iceborn Gauntlet",
	"6664": "Hollow Radiance",
	"6665": "Jak'Sho",
	"6667": "Sunfire Aegis",
	"6672": "Kraken Slayer",
	"6673": "Immortal Shieldbow",
	"6675": "Navori Flickerblade",
	"6676": "The Collector",
	"6692": "Eclipse",
	"6693": "Prowler's Claw",
	"6694": "Serylda's Grudge",
	"6695": "Serpent's Fang",
	"6696": "Axiom Arc",
	"6697": "Hubris",
	"6698": "Profane Hydra",
	"6699": "Voltaic Cyclosword",
	"6700": "Shield of the Rakkor",
	"6701": "Opportunity",

	// AD Items
	"3031": "Infinity Edge",
	"3033": "Mortal Reminder",
	"3036": "Lord Dominik's Regards",
	"3071": "Black Cleaver",
	"3072": "Bloodthirster",
	"3074": "Ravenous Hydra",
	"3077": "Tiamat",
	"3078": "Trinity Force",
	"3085": "Runaan's Hurricane",
	"3087": "Statikk Shiv",
	"3091": "Wit's End",
	"3094": "Rapid Firecannon",
	"3095": "Stormrazor",
	"3100": "Lich Bane",
	"3115": "Nashor's Tooth",
	"3116": "Rylai's Crystal Scepter",
	"3124": "Guinsoo's Rageblade",
	"3135": "Void Staff",
	"3137": "Cryptbloom",
	"3139": "Mercurial Scimitar",
	"3142": "Youmuu's Ghostblade",
	"3153": "Blade of the Ruined King",
	"3156": "Maw of Malmortius",
	"3157": "Zhonya's Hourglass",
	"3161": "Spear of Shojin",
	"3165": "Morellonomicon",
	"3179": "Umbral Glaive",

	// Tank Items
	"3001": "Abyssal Mask",
	"3002": "Trailblazer",
	"3003": "Archangel's Staff",
	"3004": "Manamune",
	"3011": "Chemtech Putrifier",
	"3041": "Mejai's Soulstealer",
	"3050": "Zeke's Convergence",
	"3065": "Spirit Visage",
	"3067": "Kindlegem",
	"3068": "Sunfire Aegis",
	"3075": "Thornmail",
	"3076": "Bramble Vest",
	"3083": "Warmog's Armor",
	"3102": "Banshee's Veil",
	"3107": "Redemption",
	"3109": "Knight's Vow",
	"3110": "Frozen Heart",
	"3119": "Winter's Approach",
	"3143": "Randuin's Omen",
	"3190": "Locket of the Iron Solari",
	"3193": "Gargoyle Stoneplate",
	"3222": "Mikael's Blessing",
	"3504": "Ardent Censer",
	"3742": "Dead Man's Plate",
	"3748": "Titanic Hydra",
	"4401": "Force of Nature",
	"4403": "The Golden Spatula",
	"2504": "Kaenic Rookern",
	"3118": "Malignance",

	// Support Items
	"3114": "Forbidden Idol",
	"3801": "Crystalline Bracer",
	"4005": "Imperial Mandate",
	"4629": "Cosmic Drive",
	"4633": "Riftmaker",
	"4636": "Night Harvester",
	"4637": "Demonic Embrace",
	"4644": "Crown of the Shattered Queen",
	"4645": "Shadowflame",
	"4646": "Stormsurge",
}

func getItemName(id string) string {
	if name, ok := itemNames[id]; ok {
		return name
	}
	// Return ID if name not found
	return fmt.Sprintf("Item #%s", id)
}
