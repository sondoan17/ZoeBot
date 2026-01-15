import discord
from discord import ui
from typing import Optional, Callable, Any


class ConfirmView(ui.View):
    """Confirmation view with Yes/No buttons."""

    def __init__(self, timeout: float = 60.0):
        super().__init__(timeout=timeout)
        self.value: Optional[bool] = None
        self.interaction: Optional[discord.Interaction] = None

    @ui.button(label="✅ Xác nhận", style=discord.ButtonStyle.success)
    async def confirm(self, interaction: discord.Interaction, button: ui.Button):
        self.value = True
        self.interaction = interaction
        self.stop()

    @ui.button(label="❌ Huỷ", style=discord.ButtonStyle.danger)
    async def cancel(self, interaction: discord.Interaction, button: ui.Button):
        self.value = False
        self.interaction = interaction
        self.stop()


class TrackPlayerView(ui.View):
    """View with button to track a player after analyzing."""

    def __init__(
        self,
        riot_id: str,
        track_callback: Callable[[discord.Interaction, str], Any],
        timeout: float = 120.0,
    ):
        super().__init__(timeout=timeout)
        self.riot_id = riot_id
        self.track_callback = track_callback

    @ui.button(label="📌 Track người chơi này", style=discord.ButtonStyle.primary)
    async def track_button(self, interaction: discord.Interaction, button: ui.Button):
        await self.track_callback(interaction, self.riot_id)
        button.disabled = True
        button.label = "✅ Đã track"
        button.style = discord.ButtonStyle.success
        await interaction.response.edit_message(view=self)


class AnalysisDetailView(ui.View):
    """View with button to show detailed analysis."""

    def __init__(
        self,
        players_data: list,
        match_data: dict,
        embed_builder: Any,  # EmbedBuilder class
        timeout: float = 300.0,
    ):
        super().__init__(timeout=timeout)
        self.players_data = players_data
        self.match_data = match_data
        self.embed_builder = embed_builder
        self.current_player_index = 0

    @ui.button(label="👤 Xem chi tiết từng người", style=discord.ButtonStyle.secondary)
    async def detail_button(self, interaction: discord.Interaction, button: ui.Button):
        if not self.players_data:
            await interaction.response.send_message("Không có dữ liệu người chơi.", ephemeral=True)
            return

        # Create detailed embed for first player
        player = self.players_data[self.current_player_index]
        embed = self.embed_builder.player_analysis(player, self.match_data)

        # Add navigation buttons
        nav_view = PlayerNavigationView(
            players_data=self.players_data,
            match_data=self.match_data,
            embed_builder=self.embed_builder,
            current_index=0,
        )

        await interaction.response.send_message(embed=embed, view=nav_view, ephemeral=True)


class PlayerNavigationView(ui.View):
    """View for navigating between player details."""

    def __init__(
        self,
        players_data: list,
        match_data: dict,
        embed_builder: Any,
        current_index: int = 0,
        timeout: float = 300.0,
    ):
        super().__init__(timeout=timeout)
        self.players_data = players_data
        self.match_data = match_data
        self.embed_builder = embed_builder
        self.current_index = current_index
        self._update_buttons()

    def _update_buttons(self):
        """Update button states based on current index."""
        self.prev_button.disabled = self.current_index <= 0
        self.next_button.disabled = self.current_index >= len(self.players_data) - 1

    def _get_current_embed(self) -> discord.Embed:
        """Get embed for current player."""
        player = self.players_data[self.current_index]
        embed = self.embed_builder.player_analysis(player, self.match_data)
        embed.set_footer(text=f"Người chơi {self.current_index + 1}/{len(self.players_data)}")
        return embed

    @ui.button(label="◀️ Trước", style=discord.ButtonStyle.secondary)
    async def prev_button(self, interaction: discord.Interaction, button: ui.Button):
        self.current_index = max(0, self.current_index - 1)
        self._update_buttons()
        embed = self._get_current_embed()
        await interaction.response.edit_message(embed=embed, view=self)

    @ui.button(label="▶️ Sau", style=discord.ButtonStyle.secondary)
    async def next_button(self, interaction: discord.Interaction, button: ui.Button):
        self.current_index = min(len(self.players_data) - 1, self.current_index + 1)
        self._update_buttons()
        embed = self._get_current_embed()
        await interaction.response.edit_message(embed=embed, view=self)


class UntrackModal(ui.Modal, title="Xác nhận huỷ theo dõi"):
    """Modal for confirming untrack action."""

    reason = ui.TextInput(
        label="Lý do (tuỳ chọn)",
        placeholder="Nhập lý do nếu muốn...",
        required=False,
        max_length=100,
    )

    def __init__(self, riot_id: str, untrack_callback: Callable):
        super().__init__()
        self.riot_id = riot_id
        self.untrack_callback = untrack_callback

    async def on_submit(self, interaction: discord.Interaction):
        await self.untrack_callback(interaction, self.riot_id, self.reason.value)


class QuickActionsView(ui.View):
    """Quick action buttons after new match notification."""

    def __init__(
        self,
        match_id: str,
        players_data: list,
        match_data: dict,
        embed_builder: Any,
        timeout: float = 600.0,
    ):
        super().__init__(timeout=timeout)
        self.match_id = match_id
        self.players_data = players_data
        self.match_data = match_data
        self.embed_builder = embed_builder

    @ui.button(label="📊 Xem phân tích đầy đủ", style=discord.ButtonStyle.primary)
    async def full_analysis(self, interaction: discord.Interaction, button: ui.Button):
        """Show full analysis in ephemeral message."""
        if not self.players_data:
            await interaction.response.send_message("Không có dữ liệu phân tích.", ephemeral=True)
            return

        embeds = []
        for player in self.players_data[:5]:  # Max 5 players
            embed = self.embed_builder.player_analysis(player, self.match_data)
            embeds.append(embed)

        await interaction.response.send_message(embeds=embeds[:10], ephemeral=True)

    @ui.button(label="🔗 Copy Match ID", style=discord.ButtonStyle.secondary)
    async def copy_match_id(self, interaction: discord.Interaction, button: ui.Button):
        """Send match ID in ephemeral message for easy copying."""
        await interaction.response.send_message(
            f"```\n{self.match_id}\n```",
            ephemeral=True,
        )
