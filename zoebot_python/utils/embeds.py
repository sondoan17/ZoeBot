"""
Embed Builder for ZoeBot
Beautiful Discord embeds with colors, thumbnails.
"""

import discord

from config import DDRAGON_CHAMPION_ICON_URL


class EmbedBuilder:
    """Builder class for creating beautiful Discord embeds."""

    # Colors
    COLOR_WIN = 0x00FF00      # Green
    COLOR_LOSE = 0xFF0000     # Red
    COLOR_INFO = 0x3498DB     # Blue
    COLOR_WARNING = 0xFFFF00  # Yellow

    @staticmethod
    def get_champion_icon(champion_name: str) -> str:
        """Get champion icon URL from Data Dragon."""
        # Handle special champion names
        name_mapping = {
            "Wukong": "MonkeyKing",
            "Cho'Gath": "Chogath",
            "Vel'Koz": "Velkoz",
            "Kha'Zix": "Khazix",
            "Kai'Sa": "Kaisa",
            "Bel'Veth": "Belveth",
            "K'Sante": "KSante",
            "Rek'Sai": "RekSai",
            "Kog'Maw": "KogMaw",
        }
        clean_name = name_mapping.get(champion_name, champion_name.replace(" ", "").replace("'", ""))
        return DDRAGON_CHAMPION_ICON_URL.format(champion=clean_name)

    @staticmethod
    def get_score_emoji(score: float) -> str:
        """Get emoji based on player score."""
        if score >= 8:
            return "🌟"
        elif score >= 6:
            return "✅"
        elif score >= 4:
            return "⚠️"
        else:
            return "❌"

    @staticmethod
    def get_position_emoji(position: str) -> str:
        """Get emoji for each position."""
        position_emojis = {
            "TOP": "🛡️",
            "JUNGLE": "🌲",
            "MIDDLE": "⚡",
            "BOTTOM": "🏹",
            "UTILITY": "💚",
            "Đường trên": "🛡️",
            "Đi rừng": "🌲",
            "Đường giữa": "⚡",
            "Xạ thủ": "🏹",
            "Hỗ trợ": "💚",
        }
        return position_emojis.get(position, "🎮")

    @classmethod
    def match_header(cls, match_data: dict) -> discord.Embed:
        """Create header embed for match analysis."""
        win = match_data.get("win", False)
        duration = match_data.get("gameDurationMinutes", 0)
        game_mode = match_data.get("gameMode", "UNKNOWN")
        match_id = match_data.get("matchId", "N/A")

        embed = discord.Embed(
            title="📊 PHÂN TÍCH TRẬN ĐẤU",
            description=f"{'🏆 **THẮNG**' if win else '💀 **THUA**'} | ⏱️ {duration} phút | 🎮 {game_mode}",
            color=cls.COLOR_WIN if win else cls.COLOR_LOSE,
        )
        embed.set_footer(text=f"Match ID: {match_id}")
        return embed

    @classmethod
    def player_analysis(
        cls,
        player_data: dict,
        match_data: dict,
        show_thumbnail: bool = True,
    ) -> discord.Embed:
        """Create embed for a single player's analysis."""
        champion = player_data.get("champion", "Unknown")
        player_name = player_data.get("player_name", "Unknown")
        position = player_data.get("position_vn", "Unknown")
        score = player_data.get("score", 0)
        win = match_data.get("win", False)

        position_emoji = cls.get_position_emoji(position)
        score_emoji = cls.get_score_emoji(score)

        embed = discord.Embed(
            title=f"{score_emoji} {champion} - {player_name}",
            description=f"{position_emoji} {position} | **{score}/10**",
            color=cls.COLOR_WIN if win else cls.COLOR_LOSE,
        )

        if show_thumbnail:
            embed.set_thumbnail(url=cls.get_champion_icon(champion))

        # Add fields
        if player_data.get("vs_opponent"):
            embed.add_field(name="⚔️ So sánh với đối thủ", value=player_data["vs_opponent"], inline=False)

        if player_data.get("role_analysis"):
            embed.add_field(name="🎭 Vai trò", value=player_data["role_analysis"], inline=True)

        if player_data.get("highlight"):
            embed.add_field(name="💪 Điểm mạnh", value=player_data["highlight"], inline=True)

        if player_data.get("weakness"):
            embed.add_field(name="📉 Điểm yếu", value=player_data["weakness"], inline=False)

        if player_data.get("comment"):
            embed.add_field(name="📝 Nhận xét", value=f"_{player_data['comment']}_", inline=False)

        if player_data.get("timeline_analysis"):
            embed.add_field(name="⏱️ Timeline", value=player_data["timeline_analysis"], inline=False)

        return embed

    @classmethod
    def compact_analysis(cls, players: list, match_data: dict) -> discord.Embed:
        """Create a single compact embed with all players."""
        win = match_data.get("win", False)
        duration = match_data.get("gameDurationMinutes", 0)
        game_mode = match_data.get("gameMode", "UNKNOWN")
        match_id = match_data.get("matchId", "N/A")

        embed = discord.Embed(
            title="📊 PHÂN TÍCH TRẬN ĐẤU",
            description=f"{'🏆 **THẮNG**' if win else '💀 **THUA**'} | ⏱️ {duration} phút | 🎮 {game_mode}",
            color=cls.COLOR_WIN if win else cls.COLOR_LOSE,
        )

        for p in players:
            champion = p.get("champion", "Unknown")
            player_name = p.get("player_name", "Unknown")
            position = p.get("position_vn", "Unknown")
            score = p.get("score", 0)
            score_emoji = cls.get_score_emoji(score)
            position_emoji = cls.get_position_emoji(position)

            # Build field value
            lines = []
            if p.get("vs_opponent"):
                lines.append(f"⚔️ {p['vs_opponent']}")
            if p.get("highlight"):
                lines.append(f"💪 {p['highlight']}")
            if p.get("weakness"):
                lines.append(f"📉 {p['weakness']}")
            if p.get("comment"):
                lines.append(f"📝 _{p['comment']}_")

            field_value = "\n".join(lines) if lines else "Không có dữ liệu"

            embed.add_field(
                name=f"{score_emoji} {champion} - {player_name} ({position_emoji} {position}) - **{score}/10**",
                value=field_value,
                inline=False,
            )

        embed.set_footer(text=f"Match ID: {match_id}")
        return embed

    @classmethod
    def tracking_list(cls, players: list[str], channel_name: str) -> discord.Embed:
        """Create embed for tracked players list."""
        if not players:
            embed = discord.Embed(
                title="📋 Danh sách theo dõi",
                description="Chưa theo dõi người chơi nào trong kênh này.\nDùng `/track` để bắt đầu.",
                color=cls.COLOR_INFO,
            )
        else:
            player_list = "\n".join(f"• **{name}**" for name in players)
            embed = discord.Embed(
                title=f"📋 Đang theo dõi ({len(players)} người)",
                description=player_list,
                color=cls.COLOR_INFO,
            )
        return embed

    @classmethod
    def success(cls, message: str, title: str = "✅ Thành công") -> discord.Embed:
        """Create success embed."""
        return discord.Embed(title=title, description=message, color=cls.COLOR_WIN)

    @classmethod
    def error(cls, message: str, title: str = "❌ Lỗi") -> discord.Embed:
        """Create error embed."""
        return discord.Embed(title=title, description=message, color=cls.COLOR_LOSE)

    @classmethod
    def warning(cls, message: str, title: str = "⚠️ Cảnh báo") -> discord.Embed:
        """Create warning embed."""
        return discord.Embed(title=title, description=message, color=cls.COLOR_WARNING)

    @classmethod
    def info(cls, message: str, title: str = "ℹ️ Thông tin") -> discord.Embed:
        """Create info embed."""
        return discord.Embed(title=title, description=message, color=cls.COLOR_INFO)

    @classmethod
    def searching(cls, riot_id: str) -> discord.Embed:
        """Create searching status embed."""
        return discord.Embed(
            title="🔍 Đang tìm kiếm...",
            description=f"Đang tìm kiếm **{riot_id}**...",
            color=cls.COLOR_INFO,
        )

    @classmethod
    def analyzing(cls, riot_id: str, match_id: str) -> discord.Embed:
        """Create analyzing status embed."""
        return discord.Embed(
            title="⏳ Đang phân tích...",
            description=f"Đang phân tích trận đấu `{match_id}` của **{riot_id}**...",
            color=cls.COLOR_INFO,
        )
