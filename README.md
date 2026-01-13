# ZoeBot - LMHT AI Analyst Discord Bot

Bot Discord tự động theo dõi trận đấu LMHT và sử dụng AI (Gemini) để phân tích, chấm điểm và đưa ra lời khuyên cho người chơi.

## 🚀 Tính năng

- **Tracking:** Theo dõi người chơi qua Riot ID (`!track Name#Tag`).
- **Real-time:** Tự động phát hiện trận đấu mới mỗi 2 phút.
- **AI Analysis:** Phân tích chỉ số, build đồ và cách chơi bằng Google Gemini.

## 🛠️ Cài đặt & Chạy Bot

### 1. Chuẩn bị Key

Bạn cần 3 key sau trong file `.env`:

- `DISCORD_TOKEN`: Từ [Discord Developer Portal](https://discord.com/developers/applications).
- `RIOT_API_KEY`: Từ [Riot Developer](https://developer.riotgames.com/).
- `GEMINI_API_KEY`: Từ [Google AI Studio](https://aistudio.google.com/).

### 2. Cấu hình Discord Bot (Quan trọng)

Để bot hoạt động, bạn cần bật **Privileged Gateway Intents**:

1. Vào [Discord Developer Portal](https://discord.com/developers/applications).
2. Chọn App của bạn -> Vào mục **Bot**.
3. Kéo xuống phần **Privileged Gateway Intents**.
4. Bật **MESSAGE CONTENT INTENT** (Gạt xanh).
5. Lưu thay đổi.

**Mời Bot vào server:**

- Vào mục **OAuth2** -> **URL Generator**.
- Chọn scope: `bot`.
- Chọn permission: `Send Messages`, `View Channels`, `Embed Links`.
- Copy link và mời vào server của bạn.

### 3. Chạy Bot

**Cách 1: Chạy trực tiếp (Python)**

```bash
pip install -r requirements.txt
python main.py
```

**Cách 2: Docker**

```bash
docker build -t zoebot .
docker run -d --env-file .env zoebot
```

## 📝 Lệnh cơ bản

- `!ping`: Kiểm tra bot.
- `!track Name#Tag`: Theo dõi người chơi (Ví dụ: `!track Faker#SKT`).
