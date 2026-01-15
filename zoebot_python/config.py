"""
ZoeBot Configuration Module
Centralized configuration and environment variable management.
"""

import os
import pathlib

from dotenv import load_dotenv

# Load environment variables
load_dotenv()


# ═══════════════════════════════════════════════════════════════════════════════
# DISCORD
# ═══════════════════════════════════════════════════════════════════════════════

DISCORD_TOKEN = os.getenv("DISCORD_TOKEN")


# ═══════════════════════════════════════════════════════════════════════════════
# RIOT API
# ═══════════════════════════════════════════════════════════════════════════════

RIOT_API_KEY = os.getenv("RIOT_API_KEY")
RIOT_BASE_URL_ACCOUNT = (
    "https://asia.api.riotgames.com"  # For PUUID (VN is in Asia/SEA)
)
RIOT_BASE_URL_MATCH = "https://sea.api.riotgames.com"  # For Matches (VN/SEA servers)


# ═══════════════════════════════════════════════════════════════════════════════
# AI / LLM API
# ═══════════════════════════════════════════════════════════════════════════════

AI_API_KEY = os.getenv("CLIPROXY_API_KEY", "")
AI_API_URL = os.getenv("CLIPROXY_API_URL")
AI_MODEL = os.getenv("CLIPROXY_MODEL")


# ═══════════════════════════════════════════════════════════════════════════════
# REDIS (Upstash)
# ═══════════════════════════════════════════════════════════════════════════════

UPSTASH_REDIS_REST_URL = os.getenv("UPSTASH_REDIS_REST_URL")
UPSTASH_REDIS_REST_TOKEN = os.getenv("UPSTASH_REDIS_REST_TOKEN")
REDIS_KEY_TRACKED_PLAYERS = "zoebot:tracked_players"


# ═══════════════════════════════════════════════════════════════════════════════
# DATA DRAGON (Champion Assets)
# ═══════════════════════════════════════════════════════════════════════════════

DDRAGON_VERSION = "14.10.1"
DDRAGON_CHAMPION_ICON_URL = f"https://ddragon.leagueoflegends.com/cdn/{DDRAGON_VERSION}/img/champion/{{champion}}.png"


# ═══════════════════════════════════════════════════════════════════════════════
# PATHS
# ═══════════════════════════════════════════════════════════════════════════════


BASE_DIR = pathlib.Path(__file__).parent
DATA_DIR = BASE_DIR / "data"
CHAMPION_DATA_FILE = DATA_DIR / "champion.json"


# ═══════════════════════════════════════════════════════════════════════════════
# VALIDATION
# ═══════════════════════════════════════════════════════════════════════════════


def validate_config():
    """Validate required configuration."""
    errors = []

    if not DISCORD_TOKEN:
        errors.append("DISCORD_TOKEN is missing")

    if not RIOT_API_KEY:
        errors.append("RIOT_API_KEY is missing")

    if not AI_API_KEY:
        errors.append("CLIPROXY_API_KEY is missing")

    if errors:
        print("⚠️ Configuration errors:")
        for error in errors:
            print(f"  - {error}")
        return False

    return True
