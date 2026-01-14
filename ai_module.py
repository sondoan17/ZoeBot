import asyncio
import json
import logging

import requests

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class AIAnalysis:
    def __init__(self, api_key):
        self.api_key = api_key
        self.api_url = "https://openrouter.ai/api/v1/chat/completions"
        # Use preset created at: https://openrouter.ai/settings/presets
        self.preset_slug = "lol-analyze"
        self.model = f"@preset/{self.preset_slug}"
        # Fallback model if preset not available
        self.fallback_model = "tngtech/tng-r1t-chimera:free"

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
        lane_matchups = match_data.get("lane_matchups", [])
        if not teammates:
            return "Error: Teammates data missing."

        # User prompt only - system prompt is configured in preset @preset/lol-analyze
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
                {"role": "user", "content": user_prompt},
            ],
            "response_format": response_schema,
        }

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "HTTP-Referer": "https://github.com/sondoan17/ZoeBot",
            "X-Title": "ZoeBot",
            "Content-Type": "application/json",
        }

        async def make_request(model_to_use):
            """Helper to make API request with specified model"""
            payload["model"] = model_to_use
            return await asyncio.to_thread(
                requests.post,
                url=self.api_url,
                headers=headers,
                data=json.dumps(payload),
            )

        try:
            # Try with preset first
            response = await make_request(self.model)

            # If preset not found (404) or error, fallback to direct model
            if response.status_code == 404 or response.status_code >= 500:
                logger.warning(
                    f"Preset {self.model} not available, using fallback model"
                )
                response = await make_request(self.fallback_model)

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
