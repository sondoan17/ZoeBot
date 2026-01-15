"""
Tracking Cog for ZoeBot
Commands: /track, /untrack, /list
"""

import discord
from discord import app_commands
from discord.ext import commands
from typing import List

from utils.embeds import EmbedBuilder
from utils.views import ConfirmView

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from main import ZoeBot


class TrackingCog(commands.Cog):
    """Cog for player tracking commands."""

    def __init__(self, bot: "ZoeBot"):
        self.bot = bot

    @property
    def riot_client(self):
        """Get riot client from bot."""
        return self.bot.riot_client

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

    @app_commands.command(name="track", description="Theo dõi người chơi - thông báo khi có trận mới")
    @app_commands.describe(riot_id="Tên người chơi (VD: Faker#KR1)")
    async def track(self, interaction: discord.Interaction, riot_id: str):
        """Track a player for new match notifications."""
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

        # Check if already tracking
        if puuid in self.tracked_players:
            existing_channel = self.tracked_players[puuid]["channel_id"]
            if existing_channel == interaction.channel_id:
                embed = EmbedBuilder.warning(
                    f"**{riot_id}** đã được theo dõi trong kênh này rồi."
                )
                await interaction.edit_original_response(embed=embed)
                return

        # Get latest match to initialize
        matches = self.riot_client.get_match_ids_by_puuid(puuid, count=1)
        last_match_id = matches[0] if matches else None

        # Add to tracking
        self.tracked_players[puuid] = {
            "last_match_id": last_match_id,
            "channel_id": interaction.channel_id,
            "name": riot_id,
        }
        self.save_tracked_players()

        embed = EmbedBuilder.success(
            f"Đã thêm **{riot_id}** vào danh sách theo dõi!\nBot sẽ thông báo khi có trận mới.",
            title="✅ Đã theo dõi",
        )
        embed.set_thumbnail(url=EmbedBuilder.get_champion_icon("Zoe"))
        await interaction.edit_original_response(embed=embed)
        print(f"Tracked: {riot_id} (PUUID: {puuid})")

    @app_commands.command(name="untrack", description="Huỷ theo dõi người chơi")
    @app_commands.describe(riot_id="Tên người chơi cần huỷ theo dõi")
    @app_commands.autocomplete(riot_id=player_autocomplete)
    async def untrack(self, interaction: discord.Interaction, riot_id: str):
        """Stop tracking a player."""
        # Validate format
        if "#" not in riot_id:
            embed = EmbedBuilder.error(
                "Sai định dạng! Vui lòng dùng: `Name#Tag` (VD: Faker#KR1)"
            )
            await interaction.response.send_message(embed=embed, ephemeral=True)
            return

        game_name, tag_line = riot_id.split("#", 1)

        # Show confirmation
        embed = EmbedBuilder.warning(
            f"Bạn có chắc muốn huỷ theo dõi **{riot_id}**?",
            title="⚠️ Xác nhận huỷ theo dõi",
        )
        view = ConfirmView(timeout=30.0)
        await interaction.response.send_message(embed=embed, view=view, ephemeral=True)

        # Wait for confirmation
        await view.wait()

        if view.value is None:
            embed = EmbedBuilder.info("Đã hết thời gian chờ.")
            await interaction.edit_original_response(embed=embed, view=None)
            return

        if not view.value:
            embed = EmbedBuilder.info("Đã huỷ thao tác.")
            await interaction.edit_original_response(embed=embed, view=None)
            return

        # Get PUUID
        puuid = self.riot_client.get_puuid_by_riot_id(game_name, tag_line)

        if puuid and puuid in self.tracked_players:
            del self.tracked_players[puuid]
            self.save_tracked_players()
            embed = EmbedBuilder.success(f"Đã huỷ theo dõi **{riot_id}**.")
            await interaction.edit_original_response(embed=embed, view=None)
            print(f"Untracked: {riot_id} (PUUID: {puuid})")
        else:
            embed = EmbedBuilder.error(
                f"Không tìm thấy **{riot_id}** trong danh sách đang theo dõi."
            )
            await interaction.edit_original_response(embed=embed, view=None)

    @app_commands.command(name="list", description="Xem danh sách người chơi đang theo dõi")
    async def list_players(self, interaction: discord.Interaction):
        """Show all tracked players in this channel."""
        channel_players = [
            data["name"]
            for puuid, data in self.tracked_players.items()
            if data["channel_id"] == interaction.channel_id
        ]

        channel_name = getattr(interaction.channel, "name", "Unknown")
        embed = EmbedBuilder.tracking_list(channel_players, channel_name or "Unknown")
        await interaction.response.send_message(embed=embed)


async def setup(bot: "ZoeBot"):
    """Setup function for loading the cog."""
    await bot.add_cog(TrackingCog(bot))
