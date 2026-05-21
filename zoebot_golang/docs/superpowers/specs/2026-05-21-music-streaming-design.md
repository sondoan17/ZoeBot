# Music Streaming — Design Spec

**Date:** 2026-05-21
**Topic:** Cập nhật pipeline phát nhạc trong voice channel từ "tải full MP3" sang "stream URL trực tiếp"
**Status:** Approved (pending user spec review)

## Mục tiêu

Giảm độ trễ giữa lúc user gọi `/play` và lúc nghe được note đầu tiên của bài hát từ ~10-20 giây xuống còn ~2 giây. Đồng thời loại bỏ phụ thuộc vào disk (không tải file MP3 về tmp folder nữa).

## Bối cảnh hiện tại

`internal/bot/music.go` đang dùng pattern cũ:

1. `yt-dlp` tải full audio về thư mục tạm (`MUSIC_TMP_DIR`) dưới dạng MP3
2. Sau khi tải xong, gọi `dgvoice.PlayAudioFile(voice, filePath, stop)` để phát
3. Phát xong xoá file và thư mục tạm

Nhược điểm: bài 5 phút có thể mất 10-20 giây tải xong mới bắt đầu phát; tốn disk IO; phải quản lý cleanup file.

## Phạm vi

**In scope:**
- Đổi pipeline phát nhạc trong `internal/bot/music.go` sang streaming
- Bỏ logic tạo/xoá thư mục tạm
- Bỏ config `MusicTmpDir` trong `internal/config/config.go`
- Bỏ entry `MUSIC_TMP_DIR` trong `.env.example`

**Out of scope (sẽ làm sau, ở các iteration khác):**
- Pause/resume, volume control, loop, shuffle
- Embed "now playing" / "queue" đẹp hơn (thumbnail, duration, requester)
- Auto-leave khi voice channel trống
- Preload URL của bài kế tiếp khi bài hiện tại sắp hết

## Yêu cầu hệ thống

`ffmpeg` phải có trong PATH hoặc tại đường dẫn `FFMPEG_PATH`. Trước đây không bắt buộc do `dgvoice.PlayAudioFile` tự gọi ffmpeg nội bộ; giờ chúng ta gọi ffmpeg trực tiếp nên binary phải tồn tại trên runtime host.

## Pipeline mới

```
yt-dlp --get-url --format bestaudio --no-playlist
        → direct audio URL + title + webpage_url (3 dòng print)
                ↓
ffmpeg -reconnect 1 -reconnect_streamed 1 -reconnect_delay_max 5
       -i <URL> -f s16le -ar 48000 -ac 2 pipe:1
        → PCM 16-bit signed little-endian, 48kHz stereo, stdout
                ↓
dgvoice.PlayAudio(voice, ffmpegStdout, stopChan)
        → encode Opus + gửi vào Discord voice
```

### Diễn giải

1. **Lấy URL + metadata (1 lần gọi yt-dlp duy nhất)**

   Args: `--get-url --format bestaudio --no-playlist --print title --print webpage_url --print url --no-warnings`. Output gồm title, webpage_url, direct audio URL trên 3 dòng. Không tải gì xuống disk.

   Khi query không phải URL (`looksLikeURL` = false), prefix `ytsearch1:` như cũ.

2. **Spawn ffmpeg**

   ```
   exec.Command(cfg.FFmpegPath,
     "-reconnect", "1",
     "-reconnect_streamed", "1",
     "-reconnect_delay_max", "5",
     "-i", track.StreamURL,
     "-f", "s16le",
     "-ar", "48000",
     "-ac", "2",
     "pipe:1",
   )
   ```

   `-reconnect` để ffmpeg tự reconnect khi network blip. Output PCM s16le 48kHz stereo — đúng format `dgvoice.PlayAudio` cần.

3. **Pipe stdout → dgvoice**

   Lấy `stdout` pipe của ffmpeg, truyền vào `dgvoice.PlayAudio(voice, stdout, stopChan)`. Hàm này block đến khi stream kết thúc tự nhiên hoặc `stopChan` nhận signal.

4. **Cleanup khi kết thúc bài**

   Sau khi `PlayAudio` return, kill ffmpeg process (đảm bảo không zombie), đóng stdout pipe. Không có file nào để xoá.

### Skip / Stop

Giữ nguyên cơ chế `stopChan` hiện tại — gửi signal vào chan, `dgvoice.PlayAudio` thoát, ffmpeg bị kill ở bước cleanup.

### Error handling

| Tình huống | Xử lý |
| --- | --- |
| `yt-dlp` fail (video private, geo-block, age-restricted, link sai) | Báo lỗi user-friendly, không enqueue |
| `ffmpeg` spawn fail (binary thiếu trên host) | Log lỗi, gửi embed báo "Phát nhạc lỗi, bài này bị skip" + ghi rõ nguyên nhân thiếu ffmpeg, sang bài tiếp theo. Không tự shutdown player vì user có thể `/leave` chủ động |
| `ffmpeg` fail giữa chừng (network drop quá lâu, codec lỗi) | Log lỗi, gửi embed báo skip bài, sang bài tiếp theo |

## Thay đổi code cụ thể

### `internal/bot/music.go`

1. **`queuedTrack` struct:** thay field `FilePath string` bằng `StreamURL string`.

2. **`prepareTrack`:**
   - Bỏ `os.MkdirAll(b.cfg.MusicTmpDir, ...)` và `os.MkdirTemp(...)`
   - Bỏ args `--extract-audio --audio-format mp3 --audio-quality 0 -o outputTemplate`
   - Thêm args `--get-url --format bestaudio --no-warnings`
   - Đổi `--print` thành 3 dòng: `title`, `webpage_url`, `url`
   - Parse 3 dòng, trả về `queuedTrack{Title, SourceURL, StreamURL, RequestedBy}`
   - Bỏ `filepath.Glob` và logic chọn file

3. **`playTrack`:**
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
           return fmt.Errorf("ffmpeg start: %w", err)
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

4. **Cleanup file references:** bỏ `os.RemoveAll(filepath.Dir(track.FilePath))` ở các vị trí:
   - `loop()` — sau khi `playTrack` return
   - `stopAndClear()` — vòng lặp xoá queued tracks
   - `close()` — vòng lặp xoá queued tracks và now-playing track

5. **Imports:** rà soát và bỏ `path/filepath` nếu không còn dùng. Giữ `os/exec`, `context`. `os` có thể vẫn cần cho các chỗ khác — kiểm tra trước khi bỏ.

### `internal/config/config.go`

- Bỏ field `MusicTmpDir string` trong struct `Config`
- Bỏ dòng `MusicTmpDir: getEnvOrDefault("MUSIC_TMP_DIR", os.TempDir())` trong `Load()`

### `.env.example`

- Bỏ dòng `# MUSIC_TMP_DIR=/tmp`

## Test plan

Unit test khó vì phụ thuộc 2 binary external (yt-dlp + ffmpeg). Test thủ công sau khi build:

- [ ] `/play <link YouTube hợp lệ>` — bot join voice, bắt đầu phát trong ≤ 3 giây
- [ ] `/play <từ khoá>` — tìm + phát, độ trễ tương tự
- [ ] `/skip` giữa bài — bài hiện tại dừng ngay, sang bài kế trong queue
- [ ] `/stop` — dừng phát, queue rỗng, bot vẫn ở voice
- [ ] `/leave` — bot rời voice, mọi process ffmpeg được kill
- [ ] `/play <link sai/private>` — báo lỗi rõ ràng, không crash, không treo player
- [ ] Phát 3 bài liên tiếp — chuyển bài tự nhiên, không có "ghost" ffmpeg process còn lại (kiểm tra bằng `Get-Process ffmpeg`)

## Rủi ro & giảm thiểu

| Rủi ro | Giảm thiểu |
| --- | --- |
| `ffmpeg` không có trên host khi deploy | Lỗi spawn rõ ràng, message "Cần cài ffmpeg" hiển thị cho user; document yêu cầu trong README sau |
| Direct URL của YouTube hết hạn nếu bài chờ trong queue lâu | Trong scope hiện tại không xử lý vì queue thường ngắn; nếu phát hiện vấn đề thực tế thì nâng cấp lên Hybrid preload sau |
| Process ffmpeg zombie nếu code lỗi | Dùng `exec.CommandContext` + `cancel()` ở `defer`, đảm bảo kill process khi function return |
