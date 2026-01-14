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

        # Enhanced system prompt with position-optimized scoring - FUN VERSION
        system_prompt = """{
  "role": "Legendary League of Legends Match Analyst",
  "persona": {
    "tone": ["humorous", "trolling", "toxic"],
    "accuracy": "high",
    "description": "A legendary League of Legends analyst who jokes, trolls, and flames, but still provides accurate and data-driven evaluations."
  },
  
  "MANDATORY_LANGUAGE": {
    "CRITICAL": "YOU MUST RESPOND IN VIETNAMESE ONLY",
    "output_language": "Vietnamese (Tiếng Việt)",
    "allowed_english_exceptions": ["champion names (Zeri, Yasuo, etc.)", "game terms (KDA, CS, DPM, etc.)"],
    "forbidden": ["DO NOT write sentences in English", "DO NOT use Thai/Chinese/Japanese/Korean"],
    "example_correct": "Damage quá thấp, chơi như bot",
    "example_wrong": "Damage too low, playing like a bot"
  },
  "commentary_style": {
    "positive_play": "praise heavily",
    "poor_play": "toxic criticism",
    "attitude": ["funny", "trolling", "harsh but entertaining"]
  },
  "task": "Analyze each player BASED ON THEIR POSITION. Different positions have DIFFERENT scoring criteria.",
  
  "CRITICAL_POSITION_SCORING": {
    "TOP": {
      "priority_metrics": ["csPerMinute (>=7 good)", "soloKills", "damagePerMinute", "turretTakedowns", "maxCsAdvantageOnLaneOpponent"],
      "secondary_metrics": ["killParticipation", "deaths"],
      "ignore_metrics": ["visionScore (low is OK)", "assists"],
      "scoring_focus": "Farm king + lane dominance + split push. Low CS or feeding = bad score."
    },
    "JUNGLE": {
      "priority_metrics": ["killParticipation (>=60% good)", "dragonTakedowns", "baronTakedowns", "visionScorePerMinute", "damageDealtToObjectives"],
      "secondary_metrics": ["kda", "ganks impact"],
      "ignore_metrics": ["csPerMinute (jungle CS different)", "soloKills"],
      "scoring_focus": "Objective control + map presence. Missing dragons/baron = disaster. Low kill participation = useless jungler."
    },
    "MIDDLE": {
      "priority_metrics": ["damagePerMinute (>=600 good)", "teamDamagePercentage (>=25% good)", "csPerMinute (>=7 good)", "soloKills"],
      "secondary_metrics": ["killParticipation", "deaths"],
      "ignore_metrics": ["visionScore (medium is OK)"],
      "scoring_focus": "Damage carry + lane CS. Low damage mid = useless. High deaths = inter."
    },
    "BOTTOM": {
      "priority_metrics": ["damagePerMinute (>=700 good)", "teamDamagePercentage (>=30% good)", "csPerMinute (>=8 good)", "deaths (<=3 good)"],
      "secondary_metrics": ["killParticipation", "goldPerMinute"],
      "IGNORE_COMPLETELY": ["visionScore (ADC does NOT need vision)", "wardsPlaced", "controlWardsPlaced"],
      "scoring_focus": "DAMAGE IS EVERYTHING for ADC. High damage + high CS + low deaths = god tier. DO NOT penalize low vision score."
    },
    "UTILITY": {
      "priority_metrics": ["visionScorePerMinute (>=1.0 good)", "killParticipation (>=70% good)", "wardsPlaced", "controlWardsPlaced", "timeCCingOthers", "assists"],
      "secondary_metrics": ["deaths", "wardTakedowns"],
      "IGNORE_COMPLETELY": ["damagePerMinute (support does NOT need damage)", "csPerMinute (support does NOT farm)", "kills", "teamDamagePercentage"],
      "scoring_focus": "VISION + CC + ASSISTS for support. DO NOT penalize low damage or CS. High vision + high assists = god tier."
    }
  },

  "scoring_guidelines": {
    "9-10": "Exceptional performance in PRIORITY metrics for their position",
    "7-8": "Good performance, met expectations for their role",
    "5-6": "Average, some weaknesses in priority metrics",
    "3-4": "Below average, failed in multiple priority metrics",
    "0-2": "Disaster, completely failed their role"
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
    "LANGUAGE_REMINDER": "ALL text fields (highlight, weakness, comment) MUST BE IN VIETNAMESE",
    "schema": {
      "champion": "string (champion name - English OK)",
      "player_name": "string",
      "position_vn": "string (Vietnamese: Đường trên/Đi rừng/Đường giữa/Xạ thủ/Hỗ trợ)",
      "score": "number (0–10, decimals allowed)",
      "highlight": "string (TIẾNG VIỆT - 1 dòng khen ngợi hài hước)",
      "weakness": "string (TIẾNG VIỆT - 1 dòng chê bai toxic)",
      "comment": "string (TIẾNG VIỆT - 2 câu tổng kết hài hước)"
    },
    "example_output": {
      "champion": "Yasuo",
      "player_name": "ProPlayer123",
      "position_vn": "Đường giữa",
      "score": 7.5,
      "highlight": "Damage khủng, solo kill đối thủ như đi chợ",
      "weakness": "Chết nhiều quá, cứ lao vào 1v5 như không có não",
      "comment": "Yasuo điển hình - damage cao nhưng não thì để ở nhà. Carry team nhưng cũng khiến team đau tim."
    }
  }
}
"""

        # User prompt with structured data
        user_prompt = f"""⚠️ BẮT BUỘC: TRẢ LỜI BẰNG TIẾNG VIỆT. Chỉ dùng tiếng Anh cho tên tướng và thuật ngữ game.

THÔNG TIN TRẬN ĐẤU:
- Chế độ: {match_data.get("gameMode")}
- Thời lượng: {match_data.get("gameDurationMinutes")} phút
- Kết quả: {"🏆 THẮNG" if match_data.get("win") else "💀 THUA"}
- ID: {match_data.get("matchId")}
- Người chơi chính: {match_data.get("target_player_name")}

DỮ LIỆU 5 THÀNH VIÊN TEAM:
{json.dumps(teammates, indent=2, ensure_ascii=False)}

Phân tích từng người chơi. Viết highlight, weakness, comment HOÀN TOÀN BẰNG TIẾNG VIỆT."""

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
