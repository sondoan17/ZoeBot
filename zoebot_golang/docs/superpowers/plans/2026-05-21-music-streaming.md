# Music Streaming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Đổi pipeline phát nhạc của ZoeBot từ "tải full MP3 → phát" sang "stream URL trực tiếp qua ffmpeg → phát" để giảm độ trễ từ ~10-20s xuống ~2s.

**Architecture:** `yt-dlp --get-url` lấy direct audio URL + metadata (không tải file) → spawn `ffmpeg` đọc URL, output Opus đóng gói trong Ogg container qua stdout → tự đọc Ogg pages từ stdout, lấy ra Opus packets, gửi qua `voice.OpusSend` chan của discordgo. Cleanup dùng `exec.CommandContext` + cancel để đảm bảo không zombie process.

**Tech Stack:** Go 1.24, `github.com/bwmarrin/discordgo`, external binaries `yt-dlp` + `ffmpeg`. **Bỏ `github.com/bwmarrin/dgvoice`** — library này cần CGO + libopus mà environment dev/prod hiện tại không có. Pipeline mới để ffmpeg lo phần encode Opus, bot chỉ parse Ogg + relay packets.

---

## File Structure

- **Create:** `internal/bot/oggopus.go` — Ogg-Opus reader + Discord sender (bỏ phụ thuộc `dgvoice`)
- **Modify:** `internal/bot/music.go` — đổi struct `queuedTrack`, viết lại `prepareTrack` và `playTrack`, bỏ logic file cleanup
- **Modify:** `internal/config/config.go` — bỏ field `MusicTmpDir`
- **Modify:** `.env.example` — bỏ entry `MUSIC_TMP_DIR`
- **Modify:** `go.mod` / `go.sum` — gỡ dependency `github.com/bwmarrin/dgvoice` và `layeh.com/gopus`

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

## Task 4: Tạo Ogg-Opus reader và viết lại `playTrack`

**Files:**
- Create: `internal/bot/oggopus.go`
- Modify: `internal/bot/music.go` (hàm `playTrack`)
- Modify: `internal/bot/music.go` (imports — bỏ `dgvoice`)

### Tại sao đổi hướng

Hướng cũ (`dgvoice.PlayAudio` đọc PCM) cần `layeh.com/gopus` — CGO binding cần libopus + C compiler. Environment hiện tại không có. Hướng mới: để ffmpeg encode Opus luôn, output Ogg-Opus container qua stdout, bot tự parse Ogg pages → lấy Opus packets → gửi vào `voice.OpusSend` chan. Pure Go, không cần CGO.

### Cấu trúc Ogg-Opus tóm tắt

Ogg stream gồm các "page". Page header bắt đầu bằng magic `"OggS"`, theo sau là:
- 1 byte version (luôn 0)
- 1 byte header_type (BOS, EOS, continuation flags)
- 8 bytes granule position
- 4 bytes serial number
- 4 bytes page sequence
- 4 bytes CRC
- 1 byte page_segments (số segment trong page, max 255)
- N bytes segment table (mỗi byte là size 0-255 của 1 segment)
- Data: tổng các segment

Một packet được tạo từ 1 hoặc nhiều segment liên tiếp. Segment có size 255 nghĩa là packet còn tiếp ở segment kế (có thể spill sang page kế nếu segment cuối page = 255). Segment có size < 255 là segment cuối của packet.

2 packet đầu của Ogg-Opus là header (`OpusHead`, `OpusTags`) — không phải audio, **phải skip**. Từ packet thứ 3 trở đi là Opus audio frames, mỗi packet là 1 frame 20ms ở config Discord chuẩn.

### Step 1: Tạo `internal/bot/oggopus.go`

Tạo file mới với nội dung:

```go
package bot

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/bwmarrin/discordgo"
)

// streamOggOpusToVoice reads Ogg-encapsulated Opus packets from r and forwards
// each audio packet to the Discord voice connection's OpusSend channel.
//
// It returns when r reaches EOF, when stop receives a value, or on a fatal
// parse error. It skips the two mandatory Ogg-Opus header packets
// (OpusHead, OpusTags) before forwarding audio.
func streamOggOpusToVoice(v *discordgo.VoiceConnection, r io.Reader, stop <-chan bool) error {
	if v == nil {
		return errors.New("voice connection is nil")
	}

	if err := v.Speaking(true); err != nil {
		return fmt.Errorf("set speaking: %w", err)
	}
	defer func() { _ = v.Speaking(false) }()

	headersSkipped := 0
	for {
		select {
		case <-stop:
			return nil
		default:
		}

		packet, err := readOggPacket(r)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if len(packet) == 0 {
			continue
		}

		if headersSkipped < 2 {
			headersSkipped++
			continue
		}

		if !v.Ready || v.OpusSend == nil {
			return errors.New("voice connection not ready")
		}

		select {
		case v.OpusSend <- packet:
		case <-stop:
			return nil
		}
	}
}

// readOggPacket reads the next Opus packet from an Ogg stream. A packet may
// span multiple Ogg pages if its final segment is 255 bytes. Returns io.EOF
// only when the stream ends cleanly between packets.
func readOggPacket(r io.Reader) ([]byte, error) {
	var packet []byte
	for {
		segments, err := readOggPage(r)
		if err != nil {
			if err == io.EOF && len(packet) == 0 {
				return nil, io.EOF
			}
			if err == io.EOF {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}

		for _, seg := range segments {
			packet = append(packet, seg.data...)
			if !seg.continued {
				return packet, nil
			}
		}
	}
}

type oggSegment struct {
	data      []byte
	continued bool // true if this segment is part of a packet that continues
}

// readOggPage reads one Ogg page and returns its segments split by lacing
// values. Each returned segment's `continued` flag is true when the next
// segment is part of the same packet.
func readOggPage(r io.Reader) ([]oggSegment, error) {
	header := make([]byte, 27)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	if string(header[0:4]) != "OggS" {
		return nil, fmt.Errorf("bad Ogg page magic: %q", header[0:4])
	}
	if header[4] != 0 {
		return nil, fmt.Errorf("unsupported Ogg version: %d", header[4])
	}

	pageSegments := int(header[26])
	segTable := make([]byte, pageSegments)
	if _, err := io.ReadFull(r, segTable); err != nil {
		return nil, err
	}

	totalData := 0
	for _, s := range segTable {
		totalData += int(s)
	}
	data := make([]byte, totalData)
	if totalData > 0 {
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
	}

	segments := make([]oggSegment, 0, len(segTable))
	offset := 0
	var current []byte
	for _, size := range segTable {
		current = append(current, data[offset:offset+int(size)]...)
		offset += int(size)
		if size < 255 {
			segments = append(segments, oggSegment{data: current, continued: false})
			current = nil
		}
	}
	if current != nil {
		segments = append(segments, oggSegment{data: current, continued: true})
	}
	return segments, nil
}

// keep encoding/binary in import (used implicitly via byte ordering decisions);
// we don't currently parse multi-byte fields beyond what's needed.
var _ = binary.LittleEndian
```

Lưu ý:
- `binary` import giữ lại sau dòng `var _` để tránh lỗi unused. Có thể bỏ sau khi mở rộng parser nếu cần thật, nhưng tạm thế cho gọn.
- Hàm này hoạt động đúng spec RFC 7845: 2 packet đầu (OpusHead + OpusTags) là header, skip; từ packet 3 trở đi là audio.
- Khi `stop` nhận signal, function trả về `nil` ngay (kể cả đang block ở `OpusSend`).

### Step 2: Thay thân hàm `playTrack` trong `internal/bot/music.go`

Hàm `playTrack` hiện tại:

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

	stop := make(chan bool, 1)
	p.mu.Lock()
	p.stopPlayback = stop
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.stopPlayback = make(chan bool, 1)
		p.mu.Unlock()
	}()

	streamErr := streamOggOpusToVoice(p.voice, stdout, stop)

	cancel()
	_ = cmd.Wait()

	if streamErr != nil {
		return fmt.Errorf("stream opus: %w", streamErr)
	}
	return nil
}
```

Giải thích args ffmpeg:
- `-vn` bỏ video stream (YouTube nguồn có thể có cả video, không cần)
- `-c:a libopus` encode Opus
- `-b:a 128k` bitrate 128kbps (cao hơn dgvoice default 64kbps một chút, chất lượng tốt hơn)
- `-ar 48000 -ac 2` khớp Discord (48kHz stereo)
- `-frame_duration 20` mỗi Opus frame 20ms (chuẩn Discord)
- `-application audio` Opus optimize cho music (không phải voice)
- `-f ogg` đóng gói Ogg-Opus

### Step 3: Bỏ import `dgvoice`

Trong `internal/bot/music.go`, block import hiện tại có:

```go
import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"

	"github.com/zoebot/internal/embeds"
)
```

Bỏ dòng `"github.com/bwmarrin/dgvoice"`. Kết quả (sau Task 5 sẽ tinh chỉnh thêm `os`, `path/filepath`):

```go
import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/zoebot/internal/embeds"
)
```

### Step 4: Verify build

Run: `go build ./...` từ `C:\Users\sondo\Desktop\ZoeBot\zoebot_golang`

Expected: PASS hoặc chỉ còn lỗi liên quan đến file cleanup (Task 5) — `os.RemoveAll(filepath.Dir(...))` còn ref `track.FilePath` cũ. Vào Task 5 để xử lý.

Nếu thấy lỗi liên quan tới `dgvoice` hoặc `gopus` thì có chỗ chưa bỏ — search lại file.

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
