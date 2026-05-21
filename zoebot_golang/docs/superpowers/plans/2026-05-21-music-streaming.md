# Music Streaming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Đổi pipeline phát nhạc của ZoeBot từ "tải full MP3 → phát" sang "stream URL trực tiếp qua ffmpeg → phát" để giảm độ trễ từ ~10-20s xuống ~2s.

**Architecture:** `yt-dlp --get-url` lấy direct audio URL + metadata (không tải file) → spawn `ffmpeg` đọc URL, output PCM s16le 48kHz stereo qua stdout → `dgvoice.PlayAudio` đọc stdout, encode Opus, gửi vào Discord voice connection. Cleanup dùng `exec.CommandContext` + cancel để đảm bảo không zombie process.

**Tech Stack:** Go 1.24, `github.com/bwmarrin/discordgo`, `github.com/bwmarrin/dgvoice`, external binaries `yt-dlp` + `ffmpeg`.

---

## File Structure

- **Modify:** `internal/bot/music.go` — đổi struct `queuedTrack`, viết lại `prepareTrack` và `playTrack`, bỏ logic file cleanup
- **Modify:** `internal/config/config.go` — bỏ field `MusicTmpDir`
- **Modify:** `.env.example` — bỏ entry `MUSIC_TMP_DIR`

Không tạo file mới. Toàn bộ thay đổi nằm trong scope của music subsystem hiện có.

---

## Task 1: Bỏ config `MusicTmpDir`

**Files:**
- Modify: `internal/config/config.go:44` (field declaration), `internal/config/config.go:85` (Load assignment)
- Modify: `.env.example:26`

- [ ] **Step 1: Bỏ field `MusicTmpDir` trong struct `Config`**

Mở `internal/config/config.go`, ở block "Paths" (gần dòng 41-45) hiện tại:

```go
	// Paths
	DataDir    string
	FFmpegPath string
	YTDLPPath  string
	MusicTmpDir string
```

Đổi thành:

```go
	// Paths
	DataDir    string
	FFmpegPath string
	YTDLPPath  string
```

- [ ] **Step 2: Bỏ assignment `MusicTmpDir` trong `Load()`**

Trong cùng file, ở block khởi tạo `cfg` (gần dòng 82-86) hiện tại:

```go
		// Paths
		DataDir:     getEnvOrDefault("DATA_DIR", "data"),
		FFmpegPath:  getEnvOrDefault("FFMPEG_PATH", "ffmpeg"),
		YTDLPPath:   getEnvOrDefault("YTDLP_PATH", "yt-dlp"),
		MusicTmpDir: getEnvOrDefault("MUSIC_TMP_DIR", os.TempDir()),
```

Đổi thành:

```go
		// Paths
		DataDir:    getEnvOrDefault("DATA_DIR", "data"),
		FFmpegPath: getEnvOrDefault("FFMPEG_PATH", "ffmpeg"),
		YTDLPPath:  getEnvOrDefault("YTDLP_PATH", "yt-dlp"),
```

- [ ] **Step 3: Bỏ entry trong `.env.example`**

Mở `.env.example`. Tìm dòng:

```
# MUSIC_TMP_DIR=/tmp
```

Xoá đúng dòng đó (giữ các comment khác như `# FFMPEG_PATH=ffmpeg`, `# YTDLP_PATH=yt-dlp`).

- [ ] **Step 4: Verify build (sẽ fail vì music.go vẫn ref `MusicTmpDir`)**

Run: `go build ./...`

Expected: FAIL với lỗi tương tự `internal/bot/music.go:219:25: b.cfg.MusicTmpDir undefined`. Đây là kết quả mong đợi — chứng tỏ field đã bị bỏ và bước tiếp theo (Task 2) sẽ sửa music.go.

**Không commit ở task này** — đợi Task 2 sửa xong music.go rồi commit cùng để tránh repo bị broken giữa 2 commit.

---

## Task 2: Đổi `queuedTrack` từ file-based sang URL-based

**Files:**
- Modify: `internal/bot/music.go:21-26` (struct definition)

- [ ] **Step 1: Đổi field `FilePath` thành `StreamURL`**

Mở `internal/bot/music.go`. Tìm struct `queuedTrack` hiện tại:

```go
type queuedTrack struct {
	Title       string
	SourceURL   string
	FilePath    string
	RequestedBy string
}
```

Đổi thành:

```go
type queuedTrack struct {
	Title       string
	SourceURL   string
	StreamURL   string
	RequestedBy string
}
```

- [ ] **Step 2: Verify build (vẫn fail vì các nơi khác còn ref `FilePath`)**

Run: `go build ./...`

Expected: FAIL với nhiều lỗi `track.FilePath undefined` và `current.FilePath undefined`. Sẽ được sửa ở Task 3.

---

## Task 3: Viết lại `prepareTrack` để lấy stream URL thay vì tải file

**Files:**
- Modify: `internal/bot/music.go:215-270` (toàn bộ hàm `prepareTrack`)

- [ ] **Step 1: Thay thân hàm `prepareTrack`**

Mở `internal/bot/music.go`. Hàm `prepareTrack` hiện tại (bắt đầu khoảng dòng 215) trông như sau:

```go
func (b *Bot) prepareTrack(query, requestedBy string) (*queuedTrack, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := os.MkdirAll(b.cfg.MusicTmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("tạo thư mục tạm thất bại: %w", err)
	}

	trackDir, err := os.MkdirTemp(b.cfg.MusicTmpDir, "zoebot-music-")
	if err != nil {
		return nil, fmt.Errorf("tạo thư mục bài hát thất bại: %w", err)
	}

	outputTemplate := filepath.Join(trackDir, "audio.%(ext)s")
	targetQuery := query
	if !looksLikeURL(query) {
		targetQuery = "ytsearch1:" + query
	}

	cmd := exec.CommandContext(
		ctx,
		b.cfg.YTDLPPath,
		"--no-playlist",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--print", "title",
		"--print", "webpage_url",
		"-o", outputTemplate,
		targetQuery,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(trackDir)
		return nil, fmt.Errorf("yt-dlp lỗi: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	lines := splitNonEmptyLines(string(out))
	if len(lines) < 2 {
		_ = os.RemoveAll(trackDir)
		return nil, fmt.Errorf("không đọc được metadata từ yt-dlp")
	}

	files, err := filepath.Glob(filepath.Join(trackDir, "audio.*"))
	if err != nil || len(files) == 0 {
		_ = os.RemoveAll(trackDir)
		return nil, fmt.Errorf("không tìm thấy file audio sau khi tải")
	}

	return &queuedTrack{
		Title:       lines[0],
		SourceURL:   lines[1],
		FilePath:    files[0],
		RequestedBy: requestedBy,
	}, nil
}
```

Thay TOÀN BỘ hàm bằng:

```go
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
```

Lưu ý:
- Timeout đổi từ 2 phút xuống 30 giây vì không tải file, chỉ lấy URL
- Bỏ toàn bộ logic mkdir/mkdtemp
- Bỏ flags `--extract-audio --audio-format mp3 --audio-quality 0 -o ...`
- Thêm `--format bestaudio --no-warnings` và `--print url` (dòng thứ 3)
- Yêu cầu output ≥ 3 dòng

---

## Task 4: Viết lại `playTrack` để stream qua ffmpeg

**Files:**
- Modify: `internal/bot/music.go:327-340` (hàm `playTrack`)

- [ ] **Step 1: Thay thân hàm `playTrack`**

Mở `internal/bot/music.go`. Hàm `playTrack` hiện tại:

```go
func (p *musicPlayer) playTrack(track *queuedTrack) error {
	stop := make(chan bool, 1)
	p.mu.Lock()
	p.stopPlayback = stop
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.stopPlayback = make(chan bool, 1)
		p.mu.Unlock()
	}()

	return dgvoice.PlayAudioFile(p.voice, track.FilePath, stop)
}
```

Thay bằng:

```go
func (p *musicPlayer) playTrack(track *queuedTrack) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, p.bot.cfg.FFmpegPath,
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-i", track.StreamURL,
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start (kiểm tra ffmpeg đã cài chưa): %w", err)
	}

	stop := make(chan bool, 1)
	p.mu.Lock()
	p.stopPlayback = stop
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.stopPlayback = make(chan bool, 1)
		p.mu.Unlock()
	}()

	dgvoice.PlayAudio(p.voice, stdout, stop)

	cancel()
	_ = cmd.Wait()
	return nil
}
```

Giải thích:
- `exec.CommandContext` + `cancel()` ở `defer` đảm bảo ffmpeg bị kill khi function return, dù có lỗi hay skip
- `cmd.StdoutPipe()` lấy reader từ stdout của ffmpeg
- `cmd.Start()` chạy ffmpeg async (khác `Run`/`CombinedOutput` là sync)
- `dgvoice.PlayAudio` (khác `PlayAudioFile` ở chỗ nhận `io.Reader` thay vì path) sẽ đọc PCM, encode Opus, gửi vào voice connection. Block đến khi reader EOF hoặc `stop` chan nhận signal.
- Sau khi `PlayAudio` return, `cancel()` kill ffmpeg, `cmd.Wait()` reap process

---

## Task 5: Bỏ tất cả file cleanup references

**Files:**
- Modify: `internal/bot/music.go` — 4 vị trí (`loop()`, `stopAndClear()`, `close()`)

- [ ] **Step 1: Bỏ cleanup trong `loop()`**

Tìm trong `loop()` (gần dòng 318) đoạn:

```go
		if err := p.playTrack(track); err != nil {
			log.Printf("play track failed: %v", err)
			_, _ = p.bot.session.ChannelMessageSendEmbed(p.channelID, embeds.Error("Phát nhạc lỗi rồi, bài này bị skip nha.", track.Title))
		}

		_ = os.RemoveAll(filepath.Dir(track.FilePath))

		p.mu.Lock()
```

Bỏ dòng `_ = os.RemoveAll(filepath.Dir(track.FilePath))` (và dòng trống dưới nó nếu thấy thừa). Kết quả:

```go
		if err := p.playTrack(track); err != nil {
			log.Printf("play track failed: %v", err)
			_, _ = p.bot.session.ChannelMessageSendEmbed(p.channelID, embeds.Error("Phát nhạc lỗi rồi, bài này bị skip nha.", track.Title))
		}

		p.mu.Lock()
```

- [ ] **Step 2: Bỏ cleanup trong `stopAndClear()`**

Tìm hàm `stopAndClear()` (gần dòng 366):

```go
func (p *musicPlayer) stopAndClear() {
	p.mu.Lock()
	queued := p.queue
	p.queue = nil
	stop := p.stopPlayback
	playing := p.playing
	p.mu.Unlock()

	for _, track := range queued {
		_ = os.RemoveAll(filepath.Dir(track.FilePath))
	}

	if playing {
		select {
		case stop <- true:
		default:
		}
	}
}
```

Bỏ luôn vòng `for _, track := range queued` (toàn bộ 3 dòng). Kết quả:

```go
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
```

Đồng thời bỏ luôn biến local `queued` vì không còn dùng (xem code trên — `queued :=` đã được xoá).

- [ ] **Step 3: Bỏ cleanup trong `close()`**

Tìm hàm `close()` (gần dòng 400):

```go
func (p *musicPlayer) close() {
	p.stopAndClear()
	if p.voice != nil {
		_ = p.voice.Disconnect()
	}
	if current := p.nowPlaying; current != nil {
		_ = os.RemoveAll(filepath.Dir(current.FilePath))
	}
	for _, track := range p.queue {
		_ = os.RemoveAll(filepath.Dir(track.FilePath))
	}
}
```

Đổi thành:

```go
func (p *musicPlayer) close() {
	p.stopAndClear()
	if p.voice != nil {
		_ = p.voice.Disconnect()
	}
}
```

- [ ] **Step 4: Dọn imports không dùng nữa**

Trong block `import` đầu file, xoá `"os"` và `"path/filepath"` nếu chúng không còn được tham chiếu. Sau khi xoá, block import sẽ trông như:

```go
import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"

	"github.com/zoebot/internal/embeds"
)
```

Lưu ý: nếu `gofmt`/`goimports` báo còn ref `os` ở đâu đó thì giữ lại. Sau khi sửa xong, chạy:

```bash
go build ./...
```

Expected: PASS, không còn lỗi compile.

---

## Task 6: Smoke test thủ công + commit

**Files:** không sửa file. Chỉ chạy bot và test.

- [ ] **Step 1: Build binary**

Run: `go build ./cmd/zoebot`

Expected: file `zoebot.exe` (hoặc `zoebot` trên Linux) được tạo, exit code 0.

- [ ] **Step 2: Chạy bot local**

Đảm bảo `.env` có `DISCORD_TOKEN`, `RIOT_API_KEY`, `CLIPROXY_API_KEY`. Đảm bảo `ffmpeg` và `yt-dlp` có trong PATH (test bằng `ffmpeg -version` và `yt-dlp --version`).

Run: `.\zoebot.exe` (Windows) hoặc `./zoebot` (Linux/Mac)

Expected: bot in log "Connected to Discord", "Bot ready: <username>".

- [ ] **Step 3: Test các kịch bản**

Vào Discord server, vào voice channel, chạy lần lượt:

| Lệnh | Expected |
| --- | --- |
| `/play <link YouTube hợp lệ>` | Bot join voice, bắt đầu phát trong ≤ 3 giây |
| `/play <từ khoá>` | Tìm + phát, độ trễ tương tự |
| `/queue` | Hiển thị "Now playing: <title>" |
| `/skip` | Bài hiện tại dừng ngay, sang bài kế (nếu có) |
| `/stop` | Dừng phát, queue rỗng, bot vẫn ở voice |
| `/leave` | Bot rời voice |
| `/play <link sai/private>` | Báo lỗi rõ ràng (không crash) |

- [ ] **Step 4: Verify không có ffmpeg zombie**

Sau khi `/leave`, kiểm tra:

Run (Windows): `Get-Process ffmpeg -ErrorAction SilentlyContinue`

Expected: không có process nào (hoặc chỉ có process không liên quan đến bot — kiểm tra bằng PID/start time). Nếu có ffmpeg zombie từ bot → có vấn đề trong cleanup, debug lại Task 4.

- [ ] **Step 5: Commit toàn bộ thay đổi**

Sau khi smoke test pass:

```bash
git add internal/bot/music.go internal/config/config.go .env.example
git commit -m "feat: stream music directly from yt-dlp/ffmpeg

Replace download-then-play pipeline with direct streaming. yt-dlp now
extracts the audio URL only (no file download); ffmpeg reads that URL
and pipes PCM into dgvoice.PlayAudio. Reduces play latency from
10-20s to ~2s and removes MUSIC_TMP_DIR / disk usage entirely."
```

---

## Self-Review Notes

- Spec yêu cầu giảm latency `/play` từ 10-20s xuống ~2s → covered bởi Task 3 (yt-dlp `--get-url`) + Task 4 (ffmpeg streaming).
- Spec yêu cầu bỏ `MusicTmpDir` config + `.env.example` entry → covered bởi Task 1.
- Spec yêu cầu bỏ logic tạo/xoá thư mục tạm trong music.go → covered bởi Task 3 (prepareTrack rewrite) + Task 5 (cleanup refs).
- Spec yêu cầu xử lý lỗi yt-dlp/ffmpeg user-friendly + skip bài → covered: Task 3 trả về error rõ ràng từ `prepareTrack`, Task 4 trả về error từ `playTrack`. `loop()` đã có sẵn handler `embeds.Error("Phát nhạc lỗi rồi, bài này bị skip nha.", track.Title)` để xử lý error trả về.
- Spec yêu cầu skip/stop dùng `stopChan` như cũ → covered: Task 4 giữ nguyên cơ chế `p.stopPlayback`.
- Spec yêu cầu không có ffmpeg zombie → covered: Task 4 dùng `exec.CommandContext` + `defer cancel()` + `cmd.Wait()`. Verify ở Task 6 Step 4.
- Test plan trong spec: 7 kịch bản → covered ở Task 6 Step 3 + Step 4.
