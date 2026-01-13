import requests
import json
import logging
import asyncio

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class AIAnalysis:
    def __init__(self, api_key):
        self.api_key = api_key
        self.api_url = "https://openrouter.ai/api/v1/chat/completions"
        self.model = "tngtech/deepseek-r1t2-chimera:free"
        
        if not api_key:
            logger.error("OpenRouter API Key is missing!")

    async def analyze_match(self, match_data):
        """
        Sends match data to OpenRouter to generate a coach-like analysis.
        Returns a formatted Discord message string.
        """
        if not self.api_key:
             return "⚠️ Lỗi: Chưa cấu hình OpenRouter API Key."

        if not match_data:
            return "Error: No match data provided."

        teammates = match_data.get('teammates')
        if not teammates:
            return "Error: Teammates data missing."

        # System prompt (instructions)
        system_prompt = """Bạn là một Huấn Luyện Viên Liên Minh Huyền Thoại chuyên nghiệp, tính cách hài hước nhưng tiêu chuẩn rất cao và khắt khe.

Nhiệm vụ: Phân tích dữ liệu trận đấu được cung cấp dưới dạng JSON và đưa ra đánh giá cho từng thành viên trong đội.

Quy tắc bắt buộc:
1. Đánh giá dựa trên chỉ số (KDA, Sát thương, Farm, Tầm nhìn).
2. Chuyển đổi Role sang Tiếng Việt: TOP -> Đường trên, JUNGLE -> Đi rừng, MIDDLE -> Đường giữa, BOTTOM -> Xạ thủ, UTILITY -> Hỗ trợ.
3. Output trả về dưới dạng JSON Array, tuyệt đối không viết thêm lời dẫn hay markdown thừa.

Cấu trúc JSON trả về cho mỗi người chơi:
{
    "champion": "Tên tướng",
    "player_name": "Tên người chơi",
    "position_vn": "Vị trí tiếng Việt",
    "score": "Điểm số (thang 10, kiểu số thực)",
    "comment": "Lời bình ngắn (tối đa 2 câu, tập trung vào phong độ, không nhắc đồ đạc)"
}"""

        # User prompt (data)
        user_prompt = f"""Dưới đây là dữ liệu trận đấu của team cần phân tích:

Thông tin trận đấu:
- Chế độ: {match_data.get('gameMode')}
- Thời lượng: {match_data.get('gameDuration')} giây
- Kết quả: {'Thắng' if match_data.get('win') else 'Thua'}
- ID trận: {match_data.get('matchId')}

Dữ liệu 5 thành viên trong team:
{json.dumps(teammates, indent=2, ensure_ascii=False)}"""

        payload = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt}
            ],
            "reasoning": {"enabled": True}
        }
        
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "HTTP-Referer": "https://github.com/sondoan17/ZoeBot",
            "X-Title": "ZoeBot",
            "Content-Type": "application/json"
        }

        try:
            # Run blocking call in a separate thread
            response = await asyncio.to_thread(
                requests.post,
                url=self.api_url,
                headers=headers,
                data=json.dumps(payload)
            )
            
            if response.status_code == 200:
                result = response.json()
                ai_content = result['choices'][0]['message']['content']
                
                # Parse JSON and format for Discord
                return self._format_discord_message(ai_content, match_data)
            else:
                logger.error(f"OpenRouter Error: {response.status_code} - {response.text}")
                return f"⚠️ Lỗi OpenRouter ({response.status_code}): {response.text}"
                
        except Exception as e:
            logger.error(f"AI Generation Error: {e}")
            return f"⚠️ Lỗi hệ thống AI: {str(e)}"

    def _format_discord_message(self, ai_content: str, match_data: dict) -> str:
        """
        Parse AI JSON response and format it for Discord display.
        """
        try:
            # Clean up potential markdown code blocks
            content = ai_content.strip()
            if content.startswith("```json"):
                content = content[7:]
            if content.startswith("```"):
                content = content[3:]
            if content.endswith("```"):
                content = content[:-3]
            content = content.strip()
            
            # Parse JSON
            players = json.loads(content)
            
            # Build Discord message
            win_status = "🏆 **THẮNG**" if match_data.get('win') else "💀 **THUA**"
            duration_mins = match_data.get('gameDuration', 0) // 60
            duration_secs = match_data.get('gameDuration', 0) % 60
            
            lines = [
                f"📊 **PHÂN TÍCH TRẬN ĐẤU** | {win_status}",
                f"⏱️ Thời lượng: {duration_mins}:{duration_secs:02d} | Mode: {match_data.get('gameMode')}",
                f"🆔 `{match_data.get('matchId')}`",
                "",
                "━━━━━━━━━━━━━━━━━━━━━━"
            ]
            
            for p in players:
                score = p.get('score', 0)
                # Emoji based on score
                if score >= 8:
                    emoji = "🌟"
                elif score >= 6:
                    emoji = "✅"
                elif score >= 4:
                    emoji = "⚠️"
                else:
                    emoji = "❌"
                
                lines.append(f"{emoji} **{p.get('champion')}** - {p.get('player_name')} ({p.get('position_vn')}) - **{score}/10**")
                lines.append(f"   _{p.get('comment')}_")
                lines.append("")
            
            return "\n".join(lines)
            
        except json.JSONDecodeError as e:
            logger.error(f"Failed to parse AI JSON: {e}")
            # Fallback: return raw content if parsing fails
            return f"📊 **Phân tích trận đấu:**\n\n{ai_content}"
        except Exception as e:
            logger.error(f"Error formatting Discord message: {e}")
            return f"⚠️ Lỗi format: {str(e)}\n\nRaw output:\n{ai_content}"
