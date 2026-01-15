"""
ZoeBot - Discord Bot for League of Legends Match Analysis
Main entry point.
"""

import logging
import discord
from discord.ext import commands, tasks

from config import DISCORD_TOKEN, validate_config
from services.riot_api import RiotAPI
from services.ai_analysis import AIAnalysis
from utils.redis import load_tracked_players, save_tracked_players
from utils.embeds import EmbedBuilder
from utils.views import QuickActionsView

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


# ═══════════════════════════════════════════════════════════════════════════════
# BOT CLASS
# ═══════════════════════════════════════════════════════════════════════════════


class ZoeBot(commands.Bot):
    """Custom bot class with shared resources."""

    def __init__(self):
        intents = discord.Intents.default()
        intents.message_content = True
        super().__init__(command_prefix="!", intents=intents)

        # Shared resources
        self.riot_client = RiotAPI()
        self.ai_client = AIAnalysis()
        self.tracked_players = load_tracked_players()
        self.analyzed_matches: dict[str, list[int]] = {}

    def save_tracked_players(self):
        """Save tracked players to Redis."""
        save_tracked_players(self.tracked_players)

    async def setup_hook(self):
        """Load cogs and sync commands."""
        await self.load_extension("cogs.tracking")
        await self.load_extension("cogs.analysis")
        logger.info("✅ Loaded cogs: tracking, analysis")

        await self.tree.sync()
        logger.info("✅ Synced slash commands")


bot = ZoeBot()


# ═══════════════════════════════════════════════════════════════════════════════
# EVENTS
# ═══════════════════════════════════════════════════════════════════════════════


@bot.event
async def on_ready():
    logger.info(f"Bot connected as {bot.user}")
    if not check_matches.is_running():
        check_matches.start()
    logger.info("Polling task started!")


# ═══════════════════════════════════════════════════════════════════════════════
# BACKGROUND TASK - Check for new matches
# ═══════════════════════════════════════════════════════════════════════════════


@tasks.loop(minutes=1.0)
async def check_matches():
    if not bot.tracked_players:
        return

    logger.info(f"🔄 Checking matches for {len(bot.tracked_players)} players...")

    for puuid, data in list(bot.tracked_players.items()):
        try:
            matches = bot.riot_client.get_match_ids_by_puuid(puuid, count=1)
            if not matches:
                logger.warning(f"No matches found for {data['name']}")
                continue

            latest_match_id = matches[0]
            old_match_id = data["last_match_id"]

            # If new match found
            if latest_match_id != old_match_id:
                bot.tracked_players[puuid]["last_match_id"] = latest_match_id
                bot.save_tracked_players()

                if old_match_id is None:
                    logger.info(f"📝 Initialized {data['name']} with match {latest_match_id}")
                    continue

                logger.info(f"🆕 New match found for {data['name']}: {latest_match_id}")

                channel_id = data["channel_id"]
                if latest_match_id in bot.analyzed_matches:
                    if channel_id in bot.analyzed_matches[latest_match_id]:
                        logger.info(f"⏭️ Match {latest_match_id} already analyzed for channel {channel_id}")
                        continue

                channel = bot.get_channel(channel_id)
                if not channel or not hasattr(channel, "send"):
                    logger.warning(f"Channel {channel_id} not found for {data['name']}")
                    continue

                # Type narrowing for text channels
                assert isinstance(channel, (discord.TextChannel, discord.Thread, discord.DMChannel))

                # Find all tracked players in this match
                players_in_match = [
                    p_data["name"]
                    for p_puuid, p_data in bot.tracked_players.items()
                    if p_data["channel_id"] == channel_id
                    and p_data["last_match_id"] == latest_match_id
                ]

                players_mention = ", ".join(f"**{name}**" for name in players_in_match)

                embed = EmbedBuilder.info(
                    f"{players_mention} vừa chơi xong trận!\n⏳ Đang phân tích...",
                    title="🚨 TRẬN MỚI",
                )
                await channel.send(embed=embed)

                match_details = bot.riot_client.get_match_details(latest_match_id)
                timeline_data = bot.riot_client.get_match_timeline(latest_match_id)
                
                if not match_details or not timeline_data:
                    embed = EmbedBuilder.error("Không thể lấy dữ liệu trận đấu từ Riot API.")
                    await channel.send(embed=embed)
                    continue

                filtered_data = bot.riot_client.parse_match_data(match_details, puuid, timeline_data)

                if not filtered_data:
                    embed = EmbedBuilder.error("Không thể xử lý dữ liệu trận đấu.")
                    await channel.send(embed=embed)
                    continue

                try:
                    analysis_result = await bot.ai_client.analyze_match_structured(filtered_data)

                    if analysis_result is None:
                        embed = EmbedBuilder.error("AI không trả về kết quả phân tích.")
                        await channel.send(embed=embed)
                        continue

                    players_data = analysis_result.get("players", [])
                    embed = EmbedBuilder.compact_analysis(players_data, filtered_data)

                    view = QuickActionsView(
                        match_id=latest_match_id,
                        players_data=players_data,
                        match_data=filtered_data,
                        embed_builder=EmbedBuilder,
                    )

                    await channel.send(embed=embed, view=view)

                    if latest_match_id not in bot.analyzed_matches:
                        bot.analyzed_matches[latest_match_id] = []
                    bot.analyzed_matches[latest_match_id].append(channel_id)

                    # Cleanup old entries
                    if len(bot.analyzed_matches) > 50:
                        oldest_key = next(iter(bot.analyzed_matches))
                        del bot.analyzed_matches[oldest_key]

                    logger.info(f"✅ Analysis sent for {players_mention}")

                except Exception as ai_error:
                    logger.error(f"AI Error for {data['name']}: {ai_error}")
                    embed = EmbedBuilder.error(f"Lỗi AI: {str(ai_error)[:200]}")
                    await channel.send(embed=embed)

        except Exception as e:
            logger.error(f"Error checking {data.get('name', puuid)}: {e}")
            try:
                err_channel = bot.get_channel(data.get("channel_id"))
                if err_channel and hasattr(err_channel, "send"):
                    embed = EmbedBuilder.error(
                        f"Lỗi khi kiểm tra trận của {data.get('name')}: {str(e)[:100]}"
                    )
                    await err_channel.send(embed=embed)  # type: ignore[union-attr]
            except Exception:
                pass


@check_matches.before_loop
async def before_check_matches():
    await bot.wait_until_ready()


# ═══════════════════════════════════════════════════════════════════════════════
# MAIN
# ═══════════════════════════════════════════════════════════════════════════════


def main():
    """Main entry point."""
    if not validate_config():
        logger.error("Configuration validation failed. Exiting.")
        return

    # Import keep_alive for hosting platforms like Render
    try:
        from scripts.keep_alive import keep_alive
        logger.info("Starting web server...")
        keep_alive()
    except ImportError:
        logger.warning("keep_alive not found, skipping web server")

    logger.info("Starting bot...")
    if DISCORD_TOKEN is None:
        logger.error("DISCORD_TOKEN is not set. Exiting.")
        return
    bot.run(DISCORD_TOKEN)


if __name__ == "__main__":
    main()
