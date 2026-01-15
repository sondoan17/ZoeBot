"""
Redis Client for ZoeBot
Handles persistence with Upstash Redis REST API.
"""

import json
import requests
import logging
from typing import Any

from config import (
    UPSTASH_REDIS_REST_URL,
    UPSTASH_REDIS_REST_TOKEN,
    REDIS_KEY_TRACKED_PLAYERS,
)

logger = logging.getLogger(__name__)


class RedisClient:
    """Client for Upstash Redis REST API."""

    def __init__(self):
        self.url = UPSTASH_REDIS_REST_URL
        self.token = UPSTASH_REDIS_REST_TOKEN
        self.enabled = bool(self.url and self.token)

        if not self.enabled:
            logger.warning("Upstash Redis not configured, using in-memory storage")

    def _request(self, command: list) -> dict | None:
        """Make a request to Upstash Redis REST API."""
        if not self.enabled:
            return None

        try:
            response = requests.post(
                self.url,
                headers={"Authorization": f"Bearer {self.token}"},
                json=command,
                timeout=10,
            )
            if response.status_code == 200:
                return response.json()
            else:
                logger.error(f"Redis error: {response.status_code} - {response.text}")
                return None
        except Exception as e:
            logger.error(f"Redis request failed: {e}")
            return None

    def get(self, key: str) -> Any | None:
        """Get value from Redis."""
        result = self._request(["GET", key])
        if result and result.get("result"):
            try:
                return json.loads(result["result"])
            except json.JSONDecodeError:
                logger.error(f"Failed to parse Redis data for key: {key}")
        return None

    def set(self, key: str, value: Any) -> bool:
        """Set value in Redis."""
        result = self._request(["SET", key, json.dumps(value)])
        return result and result.get("result") == "OK"

    def delete(self, key: str) -> bool:
        """Delete key from Redis."""
        result = self._request(["DEL", key])
        return result is not None


# Singleton instance
redis_client = RedisClient()


# ═══════════════════════════════════════════════════════════════════════════════
# TRACKED PLAYERS PERSISTENCE
# ═══════════════════════════════════════════════════════════════════════════════


def load_tracked_players() -> dict:
    """Load tracked players from Redis."""
    data = redis_client.get(REDIS_KEY_TRACKED_PLAYERS)
    if data:
        logger.info(f"📂 Loaded {len(data)} tracked players from Redis")
        return data
    return {}


def save_tracked_players(tracked_players: dict) -> bool:
    """Save tracked players to Redis."""
    success = redis_client.set(REDIS_KEY_TRACKED_PLAYERS, tracked_players)
    if success:
        logger.info(f"💾 Saved {len(tracked_players)} tracked players to Redis")
    else:
        logger.warning("Failed to save tracked players to Redis")
    return success
