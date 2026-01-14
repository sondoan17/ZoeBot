import asyncio
import json
import logging
import os

import requests

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class AIAnalysis:
    def __init__(self, api_key=None):
        # Using cliproxy API - load from environment variables
        self.api_key = api_key or os.environ.get("CLIPROXY_API_KEY", "")
        self.api_url = os.environ.get("CLIPROXY_API_URL")
        self.model = os.environ.get("CLIPROXY_MODEL")

        if not self.api_key:
            logger.error(
                "CLIPROXY_API_KEY is missing! Set it in environment variables."
            )

    async def analyze_match(self, match_data):
        """
        Sends match data to OpenRouter to generate a coach-like analysis.
        Returns a formatted Discord message string.
        """
        if not self.api_key:
            return "⚠️ Lỗi: Chưa cấu hình API Key."

        if not match_data:
            return "Error: No match data provided."

        teammates = match_data.get("teammates")
        lane_matchups = match_data.get("lane_matchups", [])
        if not teammates:
            return "Error: Teammates data missing."

        # System prompt for Zoe Bot personality
        system_prompt = """Bạn là "Zoe Bot" - một nhà phân tích trận đấu League of Legends huyền thoại. Phong cách: hài hước, trolling nhẹ, toxic vừa phải nhưng CHÍNH XÁC và KHÁCH QUAN.
═══════════════════════════════════════
📌 NGUYÊN TẮC BẮT BUỘC
═══════════════════════════════════════
1. NGÔN NGỮ: 
   - Viết HOÀN TOÀN bằng TIẾNG VIỆT
   - Chỉ dùng tiếng Anh cho: tên tướng, thuật ngữ game (KDA, CS, DPM, vision score)
2. PHÂN TÍCH KHÁCH QUAN - SO SÁNH VỚI ĐỐI THỦ CÙNG LANE:
   - So sánh trực tiếp các chỉ số: CS, damage, gold, kills, deaths
   - Ai có chỉ số tốt hơn = THẮNG LANE = điểm cao
   - Ai có chỉ số kém hơn = THUA LANE = điểm thấp
   - Chênh lệch lớn (>30% difference) = thắng/thua HARD
═══════════════════════════════════════
📌 ĐÁNH GIÁ THEO VAI TRÒ TƯỚNG (championTags)
═══════════════════════════════════════
🛡️ TANK (tags có "Tank"):
   ✅ Kỳ vọng: damageTakenOnTeamPercentage >= 20%, damageSelfMitigated cao
   ❌ Vấn đề: Tank chịu damage thấp hơn ADC/Mid = KHÔNG LÀM NHIỆM VỤ = trừ điểm nặng
   💡 Ví dụ: Sion top chỉ chịu 12% damage team trong khi Jinx chịu 25% = Sion núp sau ADC
⚔️ FIGHTER (tags có "Fighter"):
   ✅ Kỳ vọng: Cân bằng damage dealt/taken, soloKills, tham gia teamfight
   ❌ Vấn đề: Không gây damage hoặc chết quá nhiều mà không trade được
🗡️ ASSASSIN (tags có "Assassin"):
   ✅ Kỳ vọng: Damage cao (đặc biệt vào backline), deaths thấp (<=4)
   ❌ Vấn đề: Chết nhiều mà không giết được carry đối phương
🔮 MAGE (tags có "Mage"):
   ✅ Kỳ vọng: teamDamagePercentage >= 20%, poke/combo tốt
   ❌ Vấn đề: Damage thấp so với mid đối thủ
🏹 MARKSMAN (tags có "Marksman"):
   ✅ Kỳ vọng: teamDamagePercentage >= 25%, csPerMinute >= 7, deaths thấp
   ❌ Vấn đề: Damage thấp hơn ADC đối thủ, CS kém, chết nhiều
   ⚠️ KHÔNG trừ điểm vì vision score thấp - ADC không cần ward nhiều
🛟 SUPPORT (tags có "Support"):
   ✅ Kỳ vọng: visionScorePerMinute >= 1.0, killParticipation >= 60%, CC time cao
   ❌ Vấn đề: Vision thấp, không tham gia fight
   ⚠️ KHÔNG trừ điểm vì damage thấp hoặc CS thấp - Support không farm
═══════════════════════════════════════
📌 THANG ĐIỂM (0-10)
═══════════════════════════════════════
9-10: MVP - Thắng lane HARD + hoàn thành vai trò xuất sắc + carry team
7-8:  Tốt - Thắng lane hoặc hòa lane nhưng impact cao
5-6:  Trung bình - Hòa lane, làm đúng nhiệm vụ cơ bản
3-4:  Kém - Thua lane, không hoàn thành vai trò
0-2:  Thảm họa - Bị hủy diệt, gánh nặng của team
Điều chỉnh điểm:
- Thắng lane hard vs opponent: +1 đến +2
- Thua lane hard vs opponent: -1 đến -2
- Tank không tank (damage taken thấp): -1 đến -2
- ADC damage thấp hơn ADC đối thủ: -1 đến -2
═══════════════════════════════════════
📌 PHONG CÁCH BÌNH LUẬN
═══════════════════════════════════════
- Chơi TỐT → Khen mạnh, hype, công nhận skill
- Chơi TỆ → Toxic nhẹ, châm biếm, nhưng vẫn chỉ ra lỗi cụ thể
- Câu comment cuối → ĐÙA VỀ LORE của tướng đó
Ví dụ đùa lore:
- Yasuo feed: "Hasagi? Không, đây là Feedsuo. Gió thổi đi đâu thì chết ở đó."
- Thresh chơi tệ: "Warden of Souls? Anh này chỉ collect được soul của chính mình thôi."
- Jinx damage thấp: "Get Excited? Excited cái gì khi damage còn thua cả support."
- Sion không tank: "The Undead Juggernaut mà đứng sau ADC? Chắc sợ chết lần nữa."
- Ahri miss charm: "Nine-Tailed Fox mà charm ai cũng miss, chắc cả 9 đuôi đều mù."
- Lee Sin không gank: "The Blind Monk không thấy đường gank, đúng là mù thật."
═══════════════════════════════════════
📌 LƯU Ý QUAN TRỌNG
═══════════════════════════════════════
1. Luôn dựa vào DATA thực tế, không đoán mò
2. So sánh với OPPONENT cùng lane là tiêu chí quan trọng nhất
3. Kiểm tra championTags để biết kỳ vọng cho từng tướng
4. Đừng trừ điểm ADC vì vision, đừng trừ điểm Support vì damage
5. Comment cuối phải liên quan đến lore/title của tướng đó"""

        # User prompt with match data
        user_prompt = f"""THÔNG TIN TRẬN ĐẤU:
- Chế độ: {match_data.get("gameMode")}
- Thời lượng: {match_data.get("gameDurationMinutes")} phút
- Kết quả: {"🏆 THẮNG" if match_data.get("win") else "💀 THUA"}
- Người chơi chính: {match_data.get("target_player_name")}

SO SÁNH TỪNG LANE (Player vs Opponent):
{json.dumps(lane_matchups, indent=2, ensure_ascii=False)}

Phân tích 5 người chơi. So sánh với đối thủ cùng lane và kiểm tra vai trò tướng."""

        # JSON Schema for structured output
        response_schema = {
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
                                        "description": "Vị trí bằng tiếng Việt: Đường trên/Đi rừng/Đường giữa/Xạ thủ/Hỗ trợ",
                                    },
                                    "score": {
                                        "type": "number",
                                        "description": "Điểm từ 0-10",
                                    },
                                    "vs_opponent": {
                                        "type": "string",
                                        "description": "So sánh với đối thủ cùng lane (TIẾNG VIỆT)",
                                    },
                                    "role_analysis": {
                                        "type": "string",
                                        "description": "Phân tích vai trò tướng (TIẾNG VIỆT) - Tank có tank không? Carry có damage không?",
                                    },
                                    "highlight": {
                                        "type": "string",
                                        "description": "Điểm mạnh (TIẾNG VIỆT)",
                                    },
                                    "weakness": {
                                        "type": "string",
                                        "description": "Điểm yếu toxic (TIẾNG VIỆT)",
                                    },
                                    "comment": {
                                        "type": "string",
                                        "description": "Nhận xét tổng kết 2 câu (TIẾNG VIỆT)",
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

        payload = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            "temperature": 0.7,
            "max_tokens": 20000,
            "top_p": 1,
            "response_format": response_schema,
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
                ai_content = result["choices"][0]["message"]["content"]
                return self._format_discord_message(ai_content, match_data)
            else:
                logger.error(f"API Error: {response.status_code} - {response.text}")
                return f"⚠️ Lỗi API ({response.status_code}): {response.text}"

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

            # Parse JSON - now expects {players: [...]} structure
            data = json.loads(content)

            # Handle both old format (array) and new format ({players: array})
            if isinstance(data, list):
                players = data
            elif isinstance(data, dict) and "players" in data:
                players = data["players"]
            else:
                players = []

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

                if p.get("vs_opponent"):
                    lines.append(f"   ⚔️ {p.get('vs_opponent')}")
                if p.get("role_analysis"):
                    lines.append(f"   🎭 {p.get('role_analysis')}")
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
