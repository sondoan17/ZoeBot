# Deploy Bot ZoeBot

Hướng dẫn triển khai bot lên Server (VPS) hoặc Cloud để chạy 24/7.

## 1. Chạy với Docker (Khuyên dùng)

Đảm bảo bạn đã cài [Docker Desktop](https://www.docker.com/products/docker-desktop/) (trên máy tính) hoặc Docker Engine (trên VPS).

### Bước 1: Tạo Dockerfile

(Đã tạo sẵn trong thư mục gốc nếu chưa có hãy tạo file `Dockerfile` với nội dung sau):

```dockerfile
FROM python:3.9-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

CMD ["python", "main.py"]
```

### Bước 2: Build Image

Mở terminal tại thư mục dự án và chạy:

```bash
docker build -t zoebot .
```

### Bước 3: Chạy Container

Thay thế `YOUR_...` bằng key thật của bạn:

```bash
docker run -d --name zoebot \
  -e DISCORD_TOKEN="YOUR_DISCORD_TOKEN" \
  -e RIOT_API_KEY="YOUR_RIOT_KEY" \
  -e GEMINI_API_KEY="YOUR_GEMINI_KEY" \
  --restart always \
  zoebot
```

- `-d`: Chạy ngầm (detached mode).
- `--restart always`: Tự khởi động lại nếu bị crash hoặc restart máy.

---

## 2. Deploy lên Railway (Miễn phí / Giá rẻ)

1.  Đẩy code lên **GitHub Repo**.
2.  Tạo tài khoản tại [Railway.app](https://railway.app/).
3.  Chọn **New Project** -> **Deploy from GitHub repository** -> Chọn repo ZoeBot.
4.  Vào tab **Variables**, thêm 3 biến môi trường:
    - `DISCORD_TOKEN`
    - `RIOT_API_KEY`
    - `GEMINI_API_KEY`
5.  Railway sẽ tự động build và chạy bot.

---

## 3. Các lệnh Bot

- `!ping`: Kiểm tra bot sống hay chết.
- `!track Tên#Tag`: Bắt đầu theo dõi người chơi (Ví dụ: `!track Faker#SKT`).
