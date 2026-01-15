"""
AI Analysis Service for ZoeBot
Handles match analysis using LLM API.
"""

import asyncio
import json
import logging
import requests

from config import AI_API_KEY, AI_API_URL, AI_MODEL

logger = logging.getLogger(__name__)


# ═══════════════════════════════════════════════════════════════════════════════
# PROMPTS
# ═══════════════════════════════════════════════════════════════════════════════

SYSTEM_PROMPT = """Bạn là "Zoe Bot" - nhà phân tích League of Legends. Phong cách: hài hước, toxic mạnh nhưng CHÍNH XÁC.

⚠️ BẮT BUỘC: Viết TIẾNG VIỆT, chỉ dùng tiếng Anh cho tên tướng và thuật ngữ game.

📌 CÁCH CHẤM ĐIỂM (0-10):
- So sánh với OPPONENT cùng lane (CS, damage, gold, kills, deaths)
- Thắng lane = điểm cao, thua lane = điểm thấp
- 9-10: MVP carry | 7-8: Tốt | 5-6: Bình thường | 3-4: Kém | 0-2: Thảm họa

📌 VAI TRÒ TƯỚNG (xem championTags):
- Tank: phải chịu >= 20% damage team, nếu không = trừ điểm
- Marksman: damage >= 25% team, CS >= 7/min, KHÔNG trừ điểm vì vision
- Support: vision >= 1.0/min, kill participation >= 60%, KHÔNG trừ điểm vì damage/CS
- Assassin/Mage: damage phải cao hơn opponent cùng role

📌 TIMELINE (nếu có):
- Gold diff @10min: + = thắng lane, - = thua lane
- Chết early = laning yếu, chết late = positioning kém

═══════════════════════════════════════
📌 FORMAT OUTPUT (TUÂN THỦ CHÍNH XÁC)
═══════════════════════════════════════

Mỗi player PHẢI có đúng các field sau với độ dài cố định:

{
  "champion": "TênTướng",
  "player_name": "TênNgườiChơi", 
  "position_vn": "Đường trên/Đi rừng/Đường giữa/Xạ thủ/Hỗ trợ",
  "score": 7.5,
  "vs_opponent": "[MAX 80 ký tự] So sánh ngắn gọn với đối thủ. VD: Thắng lane +500 gold, hơn 30 CS",
  "role_analysis": "[MAX 60 ký tự] Hoàn thành vai trò? VD: Tank chịu 25% damage team, tốt",
  "highlight": "[MAX 50 ký tự] Điểm mạnh. VD: KDA 8/2/10 cực kỳ ổn định",
  "weakness": "[MAX 50 ký tự] Điểm yếu toxic. VD: Vision = 0, mù như Lee Sin",
  "comment": "[MAX 100 ký tự] 1-2 câu + đùa về LORE tướng. VD: Thresh kéo chuẩn, collect được 15 souls từ enemy team",
  "timeline_analysis": "[MAX 60 ký tự] Phân tích timeline. VD: Gold +800 @10min, không chết early"
}

VÍ DỤ OUTPUT CHUẨN:
{
  "players": [
    {
      "champion": "Yasuo",
      "player_name": "WindWall123",
      "position_vn": "Đường giữa",
      "score": 3.5,
      "vs_opponent": "Thua lane: -40 CS, -1500 gold so với Ahri đối thủ",
      "role_analysis": "Assassin nhưng damage chỉ 12% team, quá thấp",
      "highlight": "Có 2 solo kills early game",
      "weakness": "Chết 9 lần, feed như cho ăn buffet",
      "comment": "Hasagi? Không, đây là Feedsuo. Gió thổi đi đâu thì chết ở đó.",
      "timeline_analysis": "Gold -600 @10min, chết 3 lần trước 10 phút"
    }
  ]
}

LƯU Ý:
- KHÔNG viết dài hơn giới hạn ký tự
- KHÔNG thêm field mới
- KHÔNG bỏ field nào
- Mỗi field PHẢI có nội dung, không để trống"""


# ═══════════════════════════════════════════════════════════════════════════════
# RESPONSE SCHEMA
# ═══════════════════════════════════════════════════════════════════════════════

RESPONSE_SCHEMA = {
    "type": "json_schema",
    "json_schema": {
        "name": "match_analysis",
        "strict": True,
        "schema": {
            "type": "object",
            "properties": {
                "players": {
                    "type": "array",
                    "description": "Danh sách 5 người chơi được phân tích",
                    "items": {
                        "type": "object",
                        "properties": {
                            "champion": {
                                "type": "string",
                                "description": "Tên tướng (tiếng Anh)",
                            },
                            "player_name": {
                                "type": "string",
                                "description": "Tên người chơi",
                            },
                            "position_vn": {
                                "type": "string",
                                "description": "Vị trí bằng tiếng Việt",
                            },
                            "score": {
                                "type": "number",
                                "description": "Điểm từ 0-10",
                            },
                            "vs_opponent": {
                                "type": "string",
                                "description": "So sánh với đối thủ cùng lane",
                            },
                            "role_analysis": {
                                "type": "string",
                                "description": "Phân tích vai trò tướng",
                            },
                            "highlight": {
                                "type": "string",
                                "description": "Điểm mạnh",
                            },
                            "weakness": {
                                "type": "string",
                                "description": "Điểm yếu toxic",
                            },
                            "comment": {
                                "type": "string",
                                "description": "Nhận xét tổng kết",
                            },
                            "timeline_analysis": {
                                "type": "string",
                                "description": "Phân tích timeline",
                            },
                        },
                        "required": [
                            "champion",
                            "player_name",
                            "position_vn",
                            "score",
                            "vs_opponent",
                            "role_analysis",
                            "highlight",
                            "weakness",
                            "comment",
                            "timeline_analysis",
                        ],
                        "additionalProperties": False,
                    },
                }
            },
            "required": ["players"],
            "additionalProperties": False,
        },
    },
}


# ═══════════════════════════════════════════════════════════════════════════════
# AI ANALYSIS CLASS
# ═══════════════════════════════════════════════════════════════════════════════


class AIAnalysis:
    """AI-powered match analysis service."""

    def __init__(self, api_key: str = None):
        self.api_key = api_key or AI_API_KEY
        self.api_url = AI_API_URL
        self.model = AI_MODEL

        if self.api_key:
            logger.info(f"Loaded API Key: {self.api_key[:4]}*** (length: {len(self.api_key)})")
        else:
            logger.error("AI API Key is missing!")

        logger.info(f"API URL: {self.api_url}")
        logger.info(f"Model: {self.model}")

    def _build_timeline_text(self, timeline_insights: dict) -> str:
        """Build formatted timeline text for AI prompt."""
        if not timeline_insights:
            return ""

        fb = timeline_insights.get("first_blood")
        fb_text = (
            f"{fb.get('killer')} giết {fb.get('victim')} lúc {fb.get('time_min')} phút"
            if fb else "Không có data"
        )

        gold_diff = timeline_insights.get("gold_diff_10min", {})
        gold_diff_text = (
            "\n".join([
                f"  • {name}: {data.get('diff'):+d} gold ({data.get('position')})"
                for name, data in gold_diff.items()
            ])
            if gold_diff else "  Không có data"
        )

        deaths = timeline_insights.get("deaths_timeline", [])[:5]
        deaths_text = (
            "\n".join([
                f"  • {d.get('player')} chết lúc {d.get('time_min')} phút bởi {d.get('killer')}"
                for d in deaths
            ])
            if deaths else "  Không có deaths"
        )

        objectives = timeline_insights.get("objective_kills", [])
        obj_text = (
            "\n".join([
                f"  • {o.get('monster_type')} lúc {o.get('time_min')} phút bởi {o.get('killer')}"
                for o in objectives[:5]
            ])
            if objectives else "  Không có objectives"
        )

        plates_destroyed = timeline_insights.get("turret_plates_destroyed", 0)
        plates_lost = timeline_insights.get("turret_plates_lost", 0)

        return f"""

DIỄN BIẾN TRẬN ĐẤU (Timeline):
🩸 First Blood: {fb_text}
💰 Gold Diff @10min vs Lane Opponent:
{gold_diff_text}
💀 Deaths Timeline (5 đầu tiên của team):
{deaths_text}
🐉 Objectives:
{obj_text}
🏰 Turret Plates: Team lấy {plates_destroyed}, mất {plates_lost}"""

    def _build_user_prompt(self, match_data: dict) -> str:
        """Build user prompt from match data."""
        lane_matchups = match_data.get("lane_matchups", [])
        timeline_insights = match_data.get("timeline_insights")
        timeline_text = self._build_timeline_text(timeline_insights)

        win_text = "🏆 THẮNG" if match_data.get("win") else "💀 THUA"

        return f"""THÔNG TIN TRẬN ĐẤU:
- Chế độ: {match_data.get("gameMode")}
- Thời lượng: {match_data.get("gameDurationMinutes")} phút
- Kết quả: {win_text}
- Người chơi chính: {match_data.get("target_player_name")}

SO SÁNH TỪNG LANE (Player vs Opponent):
{json.dumps(lane_matchups, indent=2, ensure_ascii=False)}{timeline_text}

Phân tích 5 người chơi. So sánh với đối thủ cùng lane, kiểm tra vai trò tướng, và xem xét timeline data nếu có."""

    def _get_score_emoji(self, score: float) -> str:
        """Get emoji based on player score."""
        if score >= 8:
            return "🌟"
        elif score >= 6:
            return "✅"
        elif score >= 4:
            return "⚠️"
        else:
            return "❌"

    def _parse_ai_response(self, ai_content: str) -> dict | None:
        """Parse AI JSON response to dictionary."""
        try:
            content = ai_content.strip()
            if content.startswith("```json"):
                content = content[7:]
            if content.startswith("```"):
                content = content[3:]
            if content.endswith("```"):
                content = content[:-3]
            content = content.strip()

            data = json.loads(content)

            if isinstance(data, list):
                return {"players": data}
            elif isinstance(data, dict) and "players" in data:
                return data
            else:
                return {"players": []}

        except json.JSONDecodeError as e:
            logger.error(f"Failed to parse AI JSON: {e}")
            return None
        except Exception as e:
            logger.error(f"Error parsing AI response: {e}")
            return None

    async def _make_api_request(self, match_data: dict) -> dict | None:
        """Make API request to AI service."""
        user_prompt = self._build_user_prompt(match_data)

        payload = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": SYSTEM_PROMPT},
                {"role": "user", "content": user_prompt},
            ],
            "temperature": 0.7,
            "max_tokens": 20000,
            "top_p": 1,
            "response_format": RESPONSE_SCHEMA,
        }

        headers = {
            "Authorization": f"Bearer {self.api_key}",
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
                return result["choices"][0]["message"]["content"]
            else:
                logger.error(f"API Error: {response.status_code} - {response.text}")
                return None

        except Exception as e:
            logger.error(f"AI Generation Error: {e}")
            return None

    async def analyze_match(self, match_data: dict) -> str:
        """
        Analyze match and return formatted string (legacy).

        Args:
            match_data: Dictionary containing match information

        Returns:
            Formatted Discord message string
        """
        if not self.api_key:
            return "⚠️ Lỗi: Chưa cấu hình API Key."

        if not match_data or not match_data.get("teammates"):
            return "Error: Invalid match data."

        ai_content = await self._make_api_request(match_data)

        if not ai_content:
            return "⚠️ Lỗi khi gọi AI API."

        return self._format_discord_message(ai_content, match_data)

    async def analyze_match_structured(self, match_data: dict) -> dict | None:
        """
        Analyze match and return structured dict (for Embeds).

        Args:
            match_data: Dictionary containing match information

        Returns:
            Dictionary with players analysis or None on error
        """
        if not self.api_key:
            logger.error("API Key not configured")
            return None

        if not match_data or not match_data.get("teammates"):
            logger.error("Invalid match data")
            return None

        ai_content = await self._make_api_request(match_data)

        if not ai_content:
            return None

        return self._parse_ai_response(ai_content)

    def _format_discord_message(self, ai_content: str, match_data: dict) -> str:
        """Format AI response for Discord display (legacy)."""
        try:
            parsed = self._parse_ai_response(ai_content)
            if not parsed:
                return f"📊 **Phân tích trận đấu:**\n\n{ai_content}"

            players = parsed.get("players", [])

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
                emoji = self._get_score_emoji(score)

                lines.append(
                    f"{emoji} **{p.get('champion')}** - {p.get('player_name')} ({p.get('position_vn')}) - **{score}/10**"
                )

                if p.get("vs_opponent"):
                    lines.append(f"   ⚔️ {p.get('vs_opponent')}")
                if p.get("role_analysis"):
                    lines.append(f"   🎭 {p.get('role_analysis')}")
                if p.get("highlight"):
                    lines.append(f"   💪 {p.get('highlight')}")
                if p.get("weakness"):
                    lines.append(f"   📉 {p.get('weakness')}")

                lines.append(f"   📝 _{p.get('comment')}_")
                if p.get("timeline_analysis"):
                    lines.append(f"   ⏱️ {p.get('timeline_analysis')}")
                lines.append("")

            return "\n".join(lines)

        except Exception as e:
            logger.error(f"Error formatting Discord message: {e}")
            return f"⚠️ Lỗi format: {str(e)}\n\nRaw output:\n{ai_content}"
