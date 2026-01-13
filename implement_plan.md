Chào bạn, đây là bản kế hoạch kỹ thuật (Implementation Plan) chi tiết từng bước để xây dựng Bot Discord tự động phân tích trận đấu LMHT, giải quyết vấn đề "Riot không có webhook" và "tối ưu dữ liệu cho AI" mà chúng ta đã thảo luận.

---

### **Giai đoạn 1: Chuẩn bị Môi trường & Tài nguyên**

Trước khi viết code, bạn cần chuẩn bị đầy đủ các "chìa khóa" để truy cập dữ liệu.

1. **Cài đặt môi trường:**

- Ngôn ngữ: **Python** (Dễ dùng, thư viện mạnh).
- Thư viện cần thiết: `discord.py` (Bot Discord), `requests` (Gọi API), `google-generativeai` (hoặc `openai`), `python-dotenv` (Bảo mật key).

2. **Lấy API Keys:**

- **Riot API Key:** Lấy tại [developer.riotgames.com](https://developer.riotgames.com/) (Lưu ý: Key Development phải gia hạn mỗi 24h, Production Key cần đăng ký duyệt).
- **Discord Bot Token:** Lấy tại [Discord Developer Portal](https://discord.com/developers/applications).
- **AI API Key:** Lấy từ Google AI Studio (Gemini) hoặc OpenAI Platform.

---

### **Giai đoạn 2: Xây dựng Module Xử lý Dữ liệu Riot (Core Logic)**

Đây là phần quan trọng nhất để lấy và lọc dữ liệu.

**Bước 2.1: Hàm lấy thông tin người chơi (PUUID)**

- **Input:** Riot ID (Ví dụ: `Faker#SKT`).
- **API:** `GET /riot/account/v1/accounts/by-riot-id/{gameName}/{tagLine}`.
- **Output:** `puuid` (Mã định danh vĩnh viễn dùng để tra cứu trận đấu).

**Bước 2.2: Hàm lấy dữ liệu trận đấu & Lọc (Data Filtering)**

- **API:** `GET /lol/match/v5/matches/{matchId}`.
- **Logic Lọc (Quan trọng):** Viết hàm `parse_match_data(json_data)`. Dựa trên `infomatch.json`, chúng ta sẽ trích xuất các trường cụ thể sau từ `info.participants`:

  - **Identity:** `puuid`, `riotIdGameName`, `riotIdTagline`, `championName`.
  - **Role/Lane:** `teamPosition` (Chính xác hơn `role` hay `individualPosition`).
  - **KDA:** `kills`, `deaths`, `assists`, `kda`.
  - **Performance:**
    - `totalDamageDealtToChampions` (Sát thương lên tướng).
    - `totalDamageTaken` (Sát thương gánh chịu).
    - `goldEarned` (Vàng kiếm được).
    - `visionScore` (Điểm tầm nhìn).
    - `creeps`: `totalMinionsKilled` + `neutralMinionsKilled`.
  - **Build:** `item0` đến `item6`.
  - **Kết quả:** `win` (Thắng/Thua).
  - **Game Info (Context):** `gameDuration` (Thời lượng game), `gameMode` (Chế độ chơi).

- **Output:** Một object JSON tinh gọn chứa thông tin chung của trận đấu và danh sách 10 người chơi với các chỉ số trên.

---

### **Giai đoạn 3: Xây dựng Cơ chế Polling (Vòng lặp tự động)**

Giải quyết vấn đề Riot không có Webhook.

**Logic hoạt động:**

1. Bot lưu một danh sách người chơi cần theo dõi (trong Database hoặc file JSON): `{'puuid': '...', 'last_match_id': 'VN2_12345'}`.
2. Sử dụng `tasks.loop` của `discord.py` chạy mỗi 2-3 phút.
3. **Quy trình trong vòng lặp:**

- Gọi API `GET /lol/match/v5/matches/by-puuid/{puuid}/ids?count=1` để lấy ID trận mới nhất.
- So sánh: Nếu `New_ID != Old_ID` => **Có trận mới**.
- Thực hiện phân tích ngay và cập nhật lại `Old_ID`.

---

### **Giai đoạn 4: Tích hợp AI Phân tích & Chấm điểm**

**Bước 4.1: Tạo Prompt (Câu lệnh) chuẩn**

- Bạn cần tạo một prompt mẫu (Template) để gửi kèm dữ liệu JSON đã lọc.
- _Ví dụ Prompt:_ "Bạn là một huấn luyện viên LMHT chuyên nghiệp. Dựa vào dữ liệu JSON tóm tắt của trận đấu (bao gồm KDA, Sát thương, Vàng, Trang bị, Chỉ số lính), hãy:
  1.  **Tóm tắt:** Nhận xét cục diện trận đấu (Thời gian: {gameDuration}s, Mode: {gameMode}).
  2.  **Phân tích:** Đánh giá màn trình diễn của người chơi `{target_player}` (Champion: {championName}, Lane: {teamPosition}). So sánh với đối thủ cùng đường nếu cần.
  3.  **Lời khuyên:** Dựa trên build đồ (`items`) và chỉ số, đưa ra 1-2 lời khuyên cải thiện.
  4.  **Chấm điểm:** Trên thang điểm 10.
      Output format: Markdown, ngắn gọn, súc tích."

**Bước 4.2: Gửi Request**

- Gửi Prompt + JSON rút gọn tới API của AI.
- Nhận về đoạn text phân tích.

---

### **Giai đoạn 5: Tích hợp vào Bot Discord & Hiển thị**

**Bước 5.1: Lệnh đăng ký theo dõi**

- User gõ: `!track Tên#Tag`.
- Bot lấy PUUID, lưu vào danh sách theo dõi và xác nhận.

**Bước 5.2: Gửi thông báo kết quả**

- Khi vòng lặp (Phase 3) phát hiện trận mới và AI trả kết quả (Phase 4).
- Bot gửi tin nhắn vào kênh Discord chỉ định.
- **Mẹo hiển thị:** Sử dụng **Discord Embeds** để tin nhắn đẹp hơn (Màu xanh nếu thắng, Đỏ nếu thua, in đậm điểm số).

---

### **Tổng hợp Sơ đồ Luồng Dữ liệu (Workflow)**

1. **Vòng lặp (2 phút/lần):** Check Riot API xem có Match ID mới không?.
2. **Có trận mới:** -> Tải JSON chi tiết -> **Chạy hàm Lọc dữ liệu** -> Gửi JSON tinh gọn cho AI.
3. **AI Phản hồi:** -> Bot nhận văn bản -> Format lại -> Gửi vào kênh Discord.

### **Giai đoạn 6: Triển khai (Deployment & DevOps)**

Đưa bot từ chạy local lên server 24/7.

**Bước 6.1: Container hóa (Docker)**

- Tạo `Dockerfile` để đóng gói môi trường.
- Giúp bot chạy ổn định trên mọi VPS/Cloud.

**Bước 6.2: Lựa chọn Hosting**

1.  **Cloud PaaS (Railway/Render/Fly.io):** Dễ dùng, connect GitHub là chạy. Phù hợp giai đoạn đầu.
2.  **VPS (DigitalOcean/AWS):** Toàn quyền kiểm soát, rẻ lâu dài. Dùng Docker Compose để quản lý.

**Bước 6.3: Monitoring**

- Thiết lập log để theo dõi lỗi.
- Auto-restart nếu bot crash.

---

### **Code Mẫu (Cấu trúc khung)**

Dưới đây là khung sườn Python để bạn bắt đầu:

```python
import discord
from discord.ext import tasks, commands
import requests
# Import các thư viện AI và Dotenv

# Cấu hình
RIOT_API_KEY = "RGAPI-..."
DISCORD_TOKEN = "..."

# Danh sách theo dõi (Lưu tạm trong Ram, thực tế nên dùng Database)
# Format: {puuid: last_match_id}
tracked_players = {}

intents = discord.Intents.default()
intents.message_content = True
bot = commands.Bot(command_prefix='!', intents=intents)

def filter_data(full_json):
    # Code lọc dữ liệu như đã bàn ở trên
    return compact_json

async def get_ai_analysis(match_data):
    # Code gọi Gemini/GPT
    return "Kết quả phân tích..."

@tasks.loop(minutes=3.0) # Polling mỗi 3 phút
async def check_matches():
    for puuid, last_id in tracked_players.items():
        # 1. Lấy match id mới nhất từ Riot
        # 2. So sánh với last_id
        # 3. Nếu mới:
        #    - Get match detail
        #    - filter_data()
        #    - get_ai_analysis()
        #    - Gửi vào kênh discord
        pass

@bot.command()
async def track(ctx, name_tag):
    # Logic lấy PUUID từ name_tag và thêm vào tracked_players
    await ctx.send(f"Đang theo dõi {name_tag}")

@bot.event
async def on_ready():
    check_matches.start() # Khởi động vòng lặp
    print(f"Bot {bot.user} đã sẵn sàng!")

bot.run(DISCORD_TOKEN)

```
