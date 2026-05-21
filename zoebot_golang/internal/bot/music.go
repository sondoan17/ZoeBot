package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/zoebot/internal/embeds"
)

type queuedTrack struct {
	Title       string
	SourceURL   string
	StreamURL   string
	RequestedBy string
}

type musicPlayer struct {
	bot          *Bot
	guildID      string
	channelID    string
	voice        *discordgo.VoiceConnection
	queue        []*queuedTrack
	nowPlaying   *queuedTrack
	playing      bool
	processing   bool
	stopPlayback chan bool
	mu           sync.Mutex
}

func (b *Bot) handlePlay(s *discordgo.Session, i *discordgo.InteractionCreate) {
	query := strings.TrimSpace(i.ApplicationCommandData().Options[0].StringValue())
	if query == "" {
		b.respondEphemeral(s, i, embeds.Error("Thiếu link hoặc từ khoá rồi nha.", ""))
		return
	}

	voiceChannelID, err := b.findMemberVoiceChannel(i.GuildID, i.Member.User.ID)
	if err != nil {
		b.respondEphemeral(s, i, embeds.Error(err.Error(), ""))
		return
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		log.Printf("defer play response failed: %v", err)
		return
	}

	player, err := b.getOrCreateMusicPlayer(i.GuildID, voiceChannelID)
	if err != nil {
		b.followupEmbed(s, i, embeds.Error("Không join voice channel được: "+err.Error(), ""))
		return
	}

	track, err := b.prepareTrack(query, i.Member.User.Username)
	if err != nil {
		b.followupEmbed(s, i, embeds.Error("Không lấy được nhạc từ YouTube: "+err.Error(), ""))
		return
	}

	position := player.enqueue(track)
	if position == 1 {
		b.followupEmbed(s, i, embeds.Success("Đã thêm vào hàng chờ và phát ngay nhe.", fmt.Sprintf("🎵 **%s**", track.Title)))
	} else {
		b.followupEmbed(s, i, embeds.Success(fmt.Sprintf("Đã xếp hàng ở vị trí **#%d**", position), fmt.Sprintf("🎵 **%s**", track.Title)))
	}

	player.startLoop()
}

func (b *Bot) handleSkip(s *discordgo.Session, i *discordgo.InteractionCreate) {
	player := b.getMusicPlayer(i.GuildID)
	if player == nil {
		b.respondEphemeral(s, i, embeds.Error("Hiện chưa có nhạc nào đang phát.", ""))
		return
	}

	if !player.skip() {
		b.respondEphemeral(s, i, embeds.Error("Không có bài nào để skip hết á.", ""))
		return
	}

	b.respondPublic(s, i, embeds.Success("Đã skip bài hiện tại.", ""))
}

func (b *Bot) handleStopMusic(s *discordgo.Session, i *discordgo.InteractionCreate) {
	player := b.getMusicPlayer(i.GuildID)
	if player == nil {
		b.respondEphemeral(s, i, embeds.Error("Bot chưa ở voice channel nào cả.", ""))
		return
	}

	player.stopAndClear()
	b.respondPublic(s, i, embeds.Success("Đã dừng nhạc và xoá hàng chờ.", ""))
}

func (b *Bot) handleLeave(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if err := b.removeMusicPlayer(i.GuildID); err != nil {
		b.respondEphemeral(s, i, embeds.Error("Bot chưa ở voice channel nào cả.", ""))
		return
	}

	b.respondPublic(s, i, embeds.Success("Zoe out voice luôn nha.", ""))
}

func (b *Bot) handleQueue(s *discordgo.Session, i *discordgo.InteractionCreate) {
	player := b.getMusicPlayer(i.GuildID)
	if player == nil {
		b.respondEphemeral(s, i, embeds.Error("Hàng chờ đang trống nè.", ""))
		return
	}

	description := player.queueSummary()
	if description == "" {
		description = "Hàng chờ đang trống nhe."
	}

	b.respondPublic(s, i, embeds.Success(description, ""))
}

func (b *Bot) getOrCreateMusicPlayer(guildID, channelID string) (*musicPlayer, error) {
	b.musicMu.Lock()
	defer b.musicMu.Unlock()

	if player, ok := b.musicPlayers[guildID]; ok {
		if player.channelID != channelID {
			if err := player.voice.ChangeChannel(channelID, false, true); err != nil {
				return nil, err
			}
			player.channelID = channelID
		}
		return player, nil
	}

	voice, err := b.session.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return nil, err
	}

	player := &musicPlayer{
		bot:          b,
		guildID:      guildID,
		channelID:    channelID,
		voice:        voice,
		stopPlayback: make(chan bool, 1),
	}
	b.musicPlayers[guildID] = player
	return player, nil
}

func (b *Bot) getMusicPlayer(guildID string) *musicPlayer {
	b.musicMu.RLock()
	defer b.musicMu.RUnlock()
	return b.musicPlayers[guildID]
}

func (b *Bot) removeMusicPlayer(guildID string) error {
	b.musicMu.Lock()
	player, ok := b.musicPlayers[guildID]
	if ok {
		delete(b.musicPlayers, guildID)
	}
	b.musicMu.Unlock()
	if !ok {
		return errors.New("music player not found")
	}
	player.close()
	return nil
}

func (b *Bot) shutdownMusicPlayers() {
	b.musicMu.Lock()
	players := make([]*musicPlayer, 0, len(b.musicPlayers))
	for guildID, player := range b.musicPlayers {
		players = append(players, player)
		delete(b.musicPlayers, guildID)
	}
	b.musicMu.Unlock()

	for _, player := range players {
		player.close()
	}
}

func (b *Bot) findMemberVoiceChannel(guildID, userID string) (string, error) {
	guild, err := b.session.State.Guild(guildID)
	if err != nil || guild == nil {
		guild, err = b.session.Guild(guildID)
		if err != nil {
			return "", fmt.Errorf("không đọc được guild state")
		}
	}

	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			return vs.ChannelID, nil
		}
	}

	return "", fmt.Errorf("vào voice channel trước rồi gọi lại `/play` nha")
}

func (b *Bot) prepareTrack(query, requestedBy string) (*queuedTrack, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	targetQuery := query
	if !looksLikeURL(query) {
		targetQuery = "ytsearch1:" + query
	}

	cmd := exec.CommandContext(
		ctx,
		b.cfg.YTDLPPath,
		"--no-playlist",
		"--no-warnings",
		"--format", "bestaudio",
		"--print", "title",
		"--print", "webpage_url",
		"--print", "url",
		targetQuery,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp lỗi: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	lines := splitNonEmptyLines(string(out))
	if len(lines) < 3 {
		return nil, fmt.Errorf("không đọc được metadata từ yt-dlp (output: %s)", strings.TrimSpace(string(out)))
	}

	return &queuedTrack{
		Title:       lines[0],
		SourceURL:   lines[1],
		StreamURL:   lines[2],
		RequestedBy: requestedBy,
	}, nil
}

func (p *musicPlayer) enqueue(track *queuedTrack) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queue = append(p.queue, track)
	return len(p.queue)
}

func (p *musicPlayer) startLoop() {
	p.mu.Lock()
	if p.processing {
		p.mu.Unlock()
		return
	}
	p.processing = true
	p.mu.Unlock()

	go p.loop()
}

func (p *musicPlayer) loop() {
	defer func() {
		p.mu.Lock()
		p.processing = false
		p.mu.Unlock()
	}()

	for {
		track := p.nextTrack()
		if track == nil {
			return
		}

		p.mu.Lock()
		p.nowPlaying = track
		p.playing = true
		p.mu.Unlock()

		if _, err := p.bot.session.ChannelMessageSendEmbed(p.channelID, embeds.Success(fmt.Sprintf("Đang phát: **%s**", track.Title), fmt.Sprintf("Requested by %s", track.RequestedBy))); err != nil {
			log.Printf("music now playing message failed: %v", err)
		}

		if err := p.playTrack(track); err != nil {
			log.Printf("play track failed: %v", err)
			_, _ = p.bot.session.ChannelMessageSendEmbed(p.channelID, embeds.Error("Phát nhạc lỗi rồi, bài này bị skip nha.", track.Title))
		}

		p.mu.Lock()
		p.nowPlaying = nil
		p.playing = false
		p.mu.Unlock()
	}
}

func (p *musicPlayer) playTrack(track *queuedTrack) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, p.bot.cfg.FFmpegPath,
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-i", track.StreamURL,
		"-vn",
		"-c:a", "libopus",
		"-b:a", "128k",
		"-ar", "48000",
		"-ac", "2",
		"-frame_duration", "20",
		"-application", "audio",
		"-f", "ogg",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start (kiểm tra ffmpeg đã cài chưa): %w", err)
	}

	p.mu.Lock()
	p.stopPlayback = make(chan bool, 1)
	stopCh := p.stopPlayback
	p.mu.Unlock()

	go func() {
		select {
		case <-stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	streamErr := streamOggOpusToVoice(ctx, p.voice, stdout)

	cancel()
	_ = cmd.Wait()

	if streamErr != nil {
		return fmt.Errorf("stream opus: %w", streamErr)
	}
	return nil
}

func (p *musicPlayer) nextTrack() *queuedTrack {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.queue) == 0 {
		return nil
	}
	track := p.queue[0]
	p.queue = p.queue[1:]
	return track
}

func (p *musicPlayer) skip() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.playing {
		return false
	}
	select {
	case p.stopPlayback <- true:
	default:
	}
	return true
}

func (p *musicPlayer) stopAndClear() {
	p.mu.Lock()
	p.queue = nil
	stop := p.stopPlayback
	playing := p.playing
	p.mu.Unlock()

	if playing {
		select {
		case stop <- true:
		default:
		}
	}
}

func (p *musicPlayer) queueSummary() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	parts := make([]string, 0, len(p.queue)+1)
	if p.nowPlaying != nil {
		parts = append(parts, "Now playing: "+p.nowPlaying.Title)
	}
	for idx, track := range p.queue {
		parts = append(parts, fmt.Sprintf("%d. %s", idx+1, track.Title))
	}
	return strings.Join(parts, "\n")
}

func (p *musicPlayer) close() {
	p.stopAndClear()
	if p.voice != nil {
		_ = p.voice.Disconnect()
	}
}

func (b *Bot) respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

func (b *Bot) respondPublic(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

func (b *Bot) followupEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	_, _ = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
}

func looksLikeURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func splitNonEmptyLines(value string) []string {
	lines := strings.Split(value, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
