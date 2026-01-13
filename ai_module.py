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
        self.model = "xiaomi/mimo-v2-flash:free"

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

        teammates = match_data.get("teammates")
        if not teammates:
            return "Error: Teammates data missing."

        # Enhanced system prompt with multi-dimensional analysis - FUN VERSION
        system_prompt = """Bạn là một Huấn Luyện Viên Liên Minh Huyền Thoại huyền thoại (Challenger x3), tính cách HÀI HƯỚC, TROLL nhưng vẫn đánh giá chuẩn xác. 

⚠️ BẮT BUỘC: TẤT CẢ NỘI DUNG PHẢN HỒI PHẢI BẰNG TIẾNG VIỆT! Chỉ được dùng tiếng Anh cho: tên tướng, thuật ngữ game (KDA, CS, etc.), và MEME LoL (running it down, inting, gap, diff, gigachad, smurf, boosted, etc.)

PHONG CÁCH NHẬN XÉT:
- Dùng từ ngữ vui vẻ, hài hước, có thể dùng meme LoL (ví dụ: "running it down", "inting", "gap", "diff", "gigachad", "smurf", "boosted")
- Khen thì khen hết lời, chê thì chê hài hước (không toxic)
- Có thể so sánh với pro player hoặc meme (VD: "farm như Faker", "int như Tyler1", "vision như faker mid không ward")
- Dùng emoji phù hợp trong comment

NHIỆM VỤ: Phân tích TOÀN DIỆN dữ liệu trận đấu và đánh giá từng thành viên dựa trên NHIỀU CHIỀU DỮ LIỆU.

QUY TẮC PHÂN TÍCH (Bắt buộc):
1. **Combat Performance**: Đánh giá KDA, killParticipation (%), takedowns, soloKills, largestKillingSpree. Chết nhiều = trừ điểm nặng.
2. **Damage Profile**: Xem damagePerMinute, teamDamagePercentage (%). ADC/Mid phải có damage cao. Support/Tank thấp là bình thường.
3. **Laning & Economy**: csPerMinute, goldPerMinute, laneMinionsFirst10Minutes, maxCsAdvantageOnLaneOpponent. CS thấp = laning yếu.
4. **Macro & Objectives**: dragonTakedowns, baronTakedowns, turretTakedowns, damageDealtToObjectives. Jungle/Top phải tham gia objectives.
5. **Vision Control**: visionScorePerMinute, wardsPlaced, controlWardsPlaced, wardsKilled. Support phải có vision cao nhất. Jungle cũng cần vision.
6. **Mechanics**: skillshotsHit, skillshotsDodged. Nếu champion dựa vào skillshot mà hit thấp = cơ học kém.

SO SÁNH THEO VAI TRÒ:
- TOP: Farm, damage, solo kills, turret damage
- JUNGLE: Kill participation, objective control, vision, gank success
- MIDDLE: Damage, roam (kill participation), cs
- BOTTOM (ADC): Damage %, cs, deaths thấp
- UTILITY (Support): Vision, CC time, kill participation, deaths thấp

VỊ TRÍ TIẾNG VIỆT: TOP→Đường trên, JUNGLE→Đi rừng, MIDDLE→Đường giữa, BOTTOM→Xạ thủ, UTILITY→Hỗ trợ

OUTPUT: JSON Array, KHÔNG có markdown hay text thừa.
{
    "champion": "Tên tướng",
    "player_name": "Tên người chơi",
    "position_vn": "Vị trí tiếng Việt",
    "score": number (thang 10, có thể lẻ như 7.5),
    "highlight": "Điểm nổi bật nhất (1 dòng, vui vẻ hài hước)",
    "weakness": "Điểm yếu cần cải thiện (1 dòng, châm biếm nhẹ nhàng nếu có)",
    "comment": "Nhận xét tổng hợp (2 câu, HÀI HƯỚC, có thể dùng meme/slang LoL)"
}"""

        # User prompt with structured data
        user_prompt = f"""THÔNG TIN TRẬN ĐẤU:
- Chế độ: {match_data.get("gameMode")}
- Thời lượng: {match_data.get("gameDurationMinutes")} phút
- Kết quả: {"🏆 THẮNG" if match_data.get("win") else "💀 THUA"}
- ID: {match_data.get("matchId")}
- Người chơi chính: {match_data.get("target_player_name")}

DỮ LIỆU 5 THÀNH VIÊN TEAM:
{json.dumps(teammates, indent=2, ensure_ascii=False)}

Hãy phân tích chi tiết từng người chơi theo các tiêu chí đã nêu."""

        payload = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            "reasoning": {"enabled": True},
        }

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "HTTP-Referer": "https://github.com/sondoan17/ZoeBot",
            "X-Title": "ZoeBot",
            "Content-Type": "application/json",
        }

        try:
            response = await asyncio.to_thread(
                requests.post,
                url=self.api_url,
                headers=headers,
                data=json.dumps(payload),
            )

            if response.status_code == 200:
                result = response.json()
                ai_content = result["choices"][0]["message"]["content"]
                return self._format_discord_message(ai_content, match_data)
            else:
                logger.error(
                    f"OpenRouter Error: {response.status_code} - {response.text}"
                )
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
            win_status = "🏆 **THẮNG**" if match_data.get("win") else "💀 **THUA**"
            duration = match_data.get("gameDurationMinutes", 0)

            lines = [
                f"📊 **PHÂN TÍCH TRẬN ĐẤU** | {win_status}",
                f"⏱️ Thời lượng: {duration} phút | Mode: {match_data.get('gameMode')}",
                f"🆔 `{match_data.get('matchId')}`",
                "",
                "━━━━━━━━━━━━━━━━━━━━━━",
            ]

            for p in players:
                score = p.get("score", 0)
                # Emoji based on score
                if score >= 8:
                    emoji = "🌟"
                elif score >= 6:
                    emoji = "✅"
                elif score >= 4:
                    emoji = "⚠️"
                else:
                    emoji = "❌"

                lines.append(
                    f"{emoji} **{p.get('champion')}** - {p.get('player_name')} ({p.get('position_vn')}) - **{score}/10**"
                )

                if p.get("highlight"):
                    lines.append(f"   💪 {p.get('highlight')}")
                if p.get("weakness"):
                    lines.append(f"   📉 {p.get('weakness')}")

                lines.append(f"   📝 _{p.get('comment')}_")
                lines.append("")

            return "\n".join(lines)

        except json.JSONDecodeError as e:
            logger.error(f"Failed to parse AI JSON: {e}")
            return f"📊 **Phân tích trận đấu:**\n\n{ai_content}"
        except Exception as e:
            logger.error(f"Error formatting Discord message: {e}")
            return f"⚠️ Lỗi format: {str(e)}\n\nRaw output:\n{ai_content}"
