"""
Analysis Cog for ZoeBot
Commands: /ping, /analyze
"""

import logging
import discord
from discord import app_commands
from discord.ext import commands
from typing import List

from utils.embeds import EmbedBuilder
from utils.views import TrackPlayerView, AnalysisDetailView

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from main import ZoeBot

logger = logging.getLogger(__name__)


class AnalysisCog(commands.Cog):
    """Cog for match analysis commands."""

    def __init__(self, bot: "ZoeBot"):
        self.bot = bot

    @property
    def riot_client(self):
        """Get riot client from bot."""
        return self.bot.riot_client

    @property
    def ai_client(self):
        """Get AI client from bot."""
        return self.bot.ai_client

    @property
    def tracked_players(self) -> dict:
        """Get tracked players dict from bot."""
        return self.bot.tracked_players

    def save_tracked_players(self):
        """Save tracked players via bot's save function."""
        self.bot.save_tracked_players()

    async def player_autocomplete(
        self,
        interaction: discord.Interaction,
        current: str,
    ) -> List[app_commands.Choice[str]]:
        """Autocomplete for tracked players in the current channel."""
        channel_players = [
            data["name"]
            for puuid, data in self.tracked_players.items()
            if data["channel_id"] == interaction.channel_id
        ]

        # Filter by current input
        if current:
            filtered = [name for name in channel_players if current.lower() in name.lower()]
        else:
            filtered = channel_players

        # Return max 25 choices (Discord limit)
        return [
            app_commands.Choice(name=name, value=name)
            for name in filtered[:25]
        ]

    @app_commands.command(name="ping", description="Kiểm tra bot còn sống không")
    async def ping(self, interaction: discord.Interaction):
        """Check if bot is alive."""
        latency = round(self.bot.latency * 1000)
        embed = EmbedBuilder.success(
            f"🏓 Pong! Độ trễ: **{latency}ms**",
            title="✅ Bot đang hoạt động",
        )
        await interaction.response.send_message(embed=embed)

    @app_commands.command(name="analyze", description="Phân tích trận đấu gần nhất của người chơi")
    @app_commands.describe(riot_id="Tên người chơi (VD: Faker#KR1)")
    @app_commands.autocomplete(riot_id=player_autocomplete)
    async def analyze(self, interaction: discord.Interaction, riot_id: str):
        """Analyze the last match of a player."""
        # Validate format
        if "#" not in riot_id:
            embed = EmbedBuilder.error(
                "Sai định dạng! Vui lòng dùng: `Name#Tag` (VD: Faker#KR1)"
            )
            await interaction.response.send_message(embed=embed, ephemeral=True)
            return

        game_name, tag_line = riot_id.split("#", 1)

        # Send searching status
        embed = EmbedBuilder.searching(riot_id)
        await interaction.response.send_message(embed=embed)

        # Get PUUID
        puuid = self.riot_client.get_puuid_by_riot_id(game_name, tag_line)

        if not puuid:
            embed = EmbedBuilder.error(
                f"Không tìm thấy người chơi **{riot_id}**. Kiểm tra lại tên và tag."
            )
            await interaction.edit_original_response(embed=embed)
            return

        # Get latest match
        matches = self.riot_client.get_match_ids_by_puuid(puuid, count=1)
        if not matches:
            embed = EmbedBuilder.error("Người chơi này chưa đánh trận nào gần đây.")
            await interaction.edit_original_response(embed=embed)
            return

        last_match_id = matches[0]

        # Update status to analyzing
        embed = EmbedBuilder.analyzing(riot_id, last_match_id)
        await interaction.edit_original_response(embed=embed)

        # Get match details
        match_details = self.riot_client.get_match_details(last_match_id)
        timeline_data = self.riot_client.get_match_timeline(last_match_id)

        if not match_details or not timeline_data:
            embed = EmbedBuilder.error("Không thể lấy dữ liệu chi tiết của trận đấu.")
            await interaction.edit_original_response(embed=embed)
            return

        # Parse match data
        filtered_data = self.riot_client.parse_match_data(match_details, puuid, timeline_data)

        if not filtered_data:
            embed = EmbedBuilder.error("Không thể lọc dữ liệu trận đấu.")
            await interaction.edit_original_response(embed=embed)
            return

        # Get AI analysis
        try:
            analysis_result = await self.ai_client.analyze_match_structured(filtered_data)

            if analysis_result is None:
                embed = EmbedBuilder.error("AI không trả về kết quả phân tích.")
                await interaction.edit_original_response(embed=embed)
                return

            players_data = analysis_result.get("players", [])

            # Create compact embed with all players
            embed = EmbedBuilder.compact_analysis(players_data, filtered_data)

            # Check if player is already tracked
            is_tracked = puuid in self.tracked_players

            # Create view with buttons
            view = AnalysisDetailView(
                players_data=players_data,
                match_data=filtered_data,
                embed_builder=EmbedBuilder,
            )

            # Add track button if not tracked
            if not is_tracked:
                async def track_callback(inter: discord.Interaction, rid: str):
                    # Get PUUID again
                    gn, tl = rid.split("#", 1)
                    p = self.riot_client.get_puuid_by_riot_id(gn, tl)
                    if p:
                        ms = self.riot_client.get_match_ids_by_puuid(p, count=1)
                        self.tracked_players[p] = {
                            "last_match_id": ms[0] if ms else None,
                            "channel_id": inter.channel_id,
                            "name": rid,
                        }
                        self.save_tracked_players()
                        await inter.response.send_message(
                            embed=EmbedBuilder.success(f"Đã thêm **{rid}** vào danh sách theo dõi!"),
                            ephemeral=True,
                        )

                track_view = TrackPlayerView(riot_id, track_callback)
                # Merge buttons
                for item in track_view.children:
                    view.add_item(item)

            await interaction.edit_original_response(embed=embed, view=view)

        except Exception as e:
            logger.error(f"AI Error: {e}")
            embed = EmbedBuilder.error(f"Lỗi AI: {str(e)[:200]}")
            await interaction.edit_original_response(embed=embed)


async def setup(bot: "ZoeBot"):
    """Setup function for loading the cog."""
    await bot.add_cog(AnalysisCog(bot))
