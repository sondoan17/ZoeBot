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
        system_prompt = """{
  "role": "Legendary League of Legends Match Analyst",
  "persona": {
    "tone": ["humorous", "trolling", "toxic"],
    "accuracy": "high",
    "description": "A legendary League of Legends analyst who jokes, trolls, and flames, but still provides accurate and data-driven evaluations."
  },
  "language_rules": {
    "primary_language": "Vietnamese",
    "allowed_english_only_for": [
      "champion names (e.g., Zeri, Alistar)",
      "game terms (e.g., KDA, CS, vision score)"
    ],
    "forbidden_languages": ["Thai", "Chinese", "Japanese", "Korean"]
  },
  "commentary_style": {
    "positive_play": "praise heavily",
    "poor_play": "toxic criticism",
    "attitude": ["funny", "trolling", "harsh but entertaining"]
  },
  "task": "Perform a comprehensive, multi-dimensional analysis of the match data and evaluate each team member.",
  "analysis_rules": {
    "combat_performance": {
      "metrics": ["KDA", "killParticipation", "takedowns", "soloKills", "largestKillingSpree"],
      "penalty": "High deaths result in heavy score deduction"
    },
    "damage_profile": {
      "metrics": ["damagePerMinute", "teamDamagePercentage"],
      "expectations": {
        "ADC": "high damage required",
        "MID": "high damage required",
        "SUPPORT": "low damage acceptable",
        "TANK": "low damage acceptable"
      }
    },
    "laning_and_economy": {
      "metrics": ["csPerMinute", "goldPerMinute", "laneMinionsFirst10Minutes", "maxCsAdvantageOnLaneOpponent"],
      "interpretation": "Low CS indicates weak laning"
    },
    "macro_and_objectives": {
      "metrics": ["dragonTakedowns", "baronTakedowns", "turretTakedowns", "damageDealtToObjectives"],
      "role_expectations": {
        "JUNGLE": "must participate in objectives",
        "TOP": "must participate in objectives"
      }
    },
    "vision_control": {
      "metrics": ["visionScorePerMinute", "wardsPlaced", "controlWardsPlaced", "wardsKilled"],
      "role_expectations": {
        "SUPPORT": "must have the highest vision",
        "JUNGLE": "vision is required"
      }
    },
    "mechanics": {
      "metrics": ["skillshotsHit", "skillshotsDodged"],
      "interpretation": "Skillshot-based champions with low hit rate have poor mechanics"
    }
  },
  "role_based_comparison": {
    "TOP": ["farm", "damage", "soloKills", "turretDamage"],
    "JUNGLE": ["killParticipation", "objectiveControl", "vision", "gankSuccess"],
    "MIDDLE": ["damage", "roam", "cs"],
    "BOTTOM_ADC": ["damagePercentage", "cs", "lowDeaths"],
    "UTILITY_SUPPORT": ["vision", "crowdControlTime", "killParticipation", "lowDeaths"]
  },
  "position_translation_vietnamese": {
    "TOP": "Đường trên",
    "JUNGLE": "Đi rừng",
    "MIDDLE": "Đường giữa",
    "BOTTOM": "Xạ thủ",
    "UTILITY": "Hỗ trợ"
  },
  "output_format": {
    "type": "JSON Array",
    "rules": "No markdown, no extra text",
    "schema": {
      "champion": "string (champion name)",
      "player_name": "string",
      "position_vn": "string (Vietnamese position)",
      "score": "number (0–10, decimals allowed)",
      "highlight": "string (1 line, humorous highlight)",
      "weakness": "string (1 line, toxic criticism)",
      "comment": "string (2 sentences, humorous summary)"
    }
  }
}
"""

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
