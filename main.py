import os
import json
import requests
import discord
from discord.ext import commands, tasks
from dotenv import load_dotenv
from riot_module import RiotAPI
from ai_module import AIAnalysis
from keep_alive import keep_alive


# Load environment variables
load_dotenv()

DISCORD_TOKEN = os.getenv("DISCORD_TOKEN")
RIOT_API_KEY = os.getenv("RIOT_API_KEY")

# Upstash Redis configuration
UPSTASH_REDIS_REST_URL = os.getenv("UPSTASH_REDIS_REST_URL")
UPSTASH_REDIS_REST_TOKEN = os.getenv("UPSTASH_REDIS_REST_TOKEN")
REDIS_KEY = "zoebot:tracked_players"

# Bot setup
intents = discord.Intents.default()
intents.message_content = True

bot = commands.Bot(command_prefix="!", intents=intents)
riot_client = RiotAPI(RIOT_API_KEY)
ai_client = AIAnalysis()  # Will load CLIPROXY_API_KEY from environment


# ═══════════════════════════════════════════════════════════════════════════════
# UPSTASH REDIS PERSISTENCE
# ═══════════════════════════════════════════════════════════════════════════════


def redis_request(command: list) -> dict | None:
    """Make a request to Upstash Redis REST API."""
    if not UPSTASH_REDIS_REST_URL or not UPSTASH_REDIS_REST_TOKEN:
        print("⚠️ Upstash Redis not configured, using in-memory storage")
        return None

    try:
        response = requests.post(
            UPSTASH_REDIS_REST_URL,
            headers={"Authorization": f"Bearer {UPSTASH_REDIS_REST_TOKEN}"},
            json=command,
            timeout=10,
        )
        if response.status_code == 200:
            return response.json()
        else:
            print(f"⚠️ Redis error: {response.status_code} - {response.text}")
            return None
    except Exception as e:
        print(f"⚠️ Redis request failed: {e}")
        return None


def load_tracked_players() -> dict:
    """Load tracked players from Upstash Redis."""
    result = redis_request(["GET", REDIS_KEY])
    if result and result.get("result"):
        try:
            data = json.loads(result["result"])
            print(f"📂 Loaded {len(data)} tracked players from Redis")
            return data
        except json.JSONDecodeError:
            print("⚠️ Failed to parse Redis data")
    return {}


def save_tracked_players():
    """Save tracked players to Upstash Redis."""
    result = redis_request(["SET", REDIS_KEY, json.dumps(tracked_players)])
    if result and result.get("result") == "OK":
        print(f"💾 Saved {len(tracked_players)} tracked players to Redis")
    else:
        print("⚠️ Failed to save to Redis")


# Tracking Data - Load from Redis on startup
# Format: {puuid: {'last_match_id': str, 'channel_id': int, 'name': str}}
tracked_players = load_tracked_players()


@bot.event
async def on_ready():
    print(f"Bot connected as {bot.user}")
    if not check_matches.is_running():
        check_matches.start()
    print("Polling task started!")


@bot.command()
async def ping(ctx):
    await ctx.send("Pong!")


@bot.command()
async def track(ctx, *, riot_id: str):
    """
    Track a player. Format: !track Name#Tag (Supports spaces)
    """
    try:
        if "#" not in riot_id:
            await ctx.send(
                "❌ Sai định dạng! Vui lòng dùng: `Name#Tag` (VD: Faker#SKT)"
            )
            return

        game_name, tag_line = riot_id.split("#", 1)
        await ctx.send(f"🔍 Đang tìm kiếm **{game_name}** #{tag_line}...")

        puuid = riot_client.get_puuid_by_riot_id(game_name, tag_line)

        if not puuid:
            await ctx.send("❌ Không tìm thấy người chơi này. Kiểm tra lại tên và tag.")
            return

        # Get latest match to initialize
        matches = riot_client.get_match_ids_by_puuid(puuid, count=1)
        last_match_id = matches[0] if matches else None

        tracked_players[puuid] = {
            "last_match_id": last_match_id,
            "channel_id": ctx.channel.id,
            "name": riot_id,
        }
        save_tracked_players()  # Persist to file

        await ctx.send(
            f"✅ Đã thêm **{riot_id}** vào danh sách theo dõi!\nBot sẽ thông báo khi có trận mới."
        )
        print(f"Tracked: {riot_id} (PUUID: {puuid})")

    except Exception as e:
        await ctx.send(f"⚠️ Có lỗi xảy ra: {str(e)}")


@bot.command()
async def untrack(ctx, *, riot_id: str):
    """
    Stop tracking a player. Format: !untrack Name#Tag
    """
    try:
        if "#" not in riot_id:
            await ctx.send(
                "❌ Sai định dạng! Vui lòng dùng: `Name#Tag` (VD: Faker#SKT)"
            )
            return

        game_name, tag_line = riot_id.split("#", 1)

        # Check if we can find them by PUUID (most accurate)
        puuid = riot_client.get_puuid_by_riot_id(game_name, tag_line)

        if puuid and puuid in tracked_players:
            del tracked_players[puuid]
            save_tracked_players()  # Persist to file
            await ctx.send(f"✅ Đã huỷ theo dõi **{riot_id}**.")
            print(f"Untracked: {riot_id} (PUUID: {puuid})")
        else:
            await ctx.send(
                f"❌ Không tìm thấy **{riot_id}** trong danh sách đang theo dõi."
            )

    except Exception as e:
        await ctx.send(f"⚠️ Có lỗi xảy ra: {str(e)}")


@bot.command(aliases=["review", "phantich"])
async def analyze(ctx, *, riot_id: str):
    """
    Analyze the last match of a player. Format: !analyze Name#Tag
    """
    try:
        if "#" not in riot_id:
            await ctx.send(
                "❌ Sai định dạng! Vui lòng dùng: `Name#Tag` (VD: Faker#SKT)"
            )
            return

        game_name, tag_line = riot_id.split("#", 1)
        await ctx.send(
            f"🔍 Đang tìm kiếm trận đấu gần nhất của **{game_name}** #{tag_line}..."
        )

        puuid = riot_client.get_puuid_by_riot_id(game_name, tag_line)

        if not puuid:
            await ctx.send(
                f"❌ Không tìm thấy người chơi **{riot_id}**. Kiểm tra lại tên và tag."
            )
            return

        # Get latest match
        matches = riot_client.get_match_ids_by_puuid(puuid, count=1)
        if not matches:
            await ctx.send("❌ Người chơi này chưa đánh trận nào gần đây.")
            return

        last_match_id = matches[0]
        await ctx.send(
            f"⏳ Đang phân tích trận đấu `{last_match_id}` của **{riot_id}**..."
        )

        match_details = riot_client.get_match_details(last_match_id)
        timeline_data = riot_client.get_match_timeline(last_match_id)
        if match_details:
            filtered_data = riot_client.parse_match_data(
                match_details, puuid, timeline_data
            )
            if filtered_data:
                analysis = await ai_client.analyze_match(filtered_data)
                if len(analysis) > 2000:
                    for i in range(0, len(analysis), 2000):
                        await ctx.send(analysis[i : i + 2000])
                else:
                    await ctx.send(analysis)
            else:
                await ctx.send("⚠️ Không thể lọc dữ liệu trận đấu.")
        else:
            await ctx.send("⚠️ Không thể lấy dữ liệu chi tiết của trận đấu.")

    except Exception as e:
        await ctx.send(f"⚠️ Có lỗi xảy ra: {str(e)}")


@tasks.loop(minutes=1.0)
async def check_matches():
    if not tracked_players:
        return

    print(f"🔄 Checking matches for {len(tracked_players)} players...")

    # Iterate copy of items to avoid modification issues during iteration
    for puuid, data in list(tracked_players.items()):
        try:
            matches = riot_client.get_match_ids_by_puuid(puuid, count=1)
            if not matches:
                print(f"⚠️ No matches found for {data['name']}")
                continue

            latest_match_id = matches[0]
            old_match_id = data["last_match_id"]

            # If new match found (and we had a previous record to compare)
            if latest_match_id != old_match_id:
                # Update first to prevent spam if processing fails
                tracked_players[puuid]["last_match_id"] = latest_match_id

                if old_match_id is None:
                    # First run/init, just update
                    print(f"📝 Initialized {data['name']} with match {latest_match_id}")
                    continue

                print(f"🆕 New match found for {data['name']}: {latest_match_id}")

                # Fetch details
                channel_id = data["channel_id"]
                channel = bot.get_channel(channel_id)
                if not channel:
                    print(f"⚠️ Channel {channel_id} not found for {data['name']}")
                    continue

                await channel.send(
                    f"🚨 **TRẬN MỚI:** {data['name']} vừa chơi xong trận `{latest_match_id}`!\n⏳ Đang phân tích..."
                )

                # Fetch match details and timeline
                match_details = riot_client.get_match_details(latest_match_id)
                if not match_details:
                    await channel.send("⚠️ Không thể lấy dữ liệu trận đấu từ Riot API.")
                    continue

                timeline_data = riot_client.get_match_timeline(latest_match_id)

                filtered_data = riot_client.parse_match_data(
                    match_details, puuid, timeline_data
                )
                if not filtered_data:
                    await channel.send("⚠️ Không thể xử lý dữ liệu trận đấu.")
                    continue

                # Get AI analysis
                try:
                    analysis = await ai_client.analyze_match(filtered_data)

                    # Handle long messages (Discord limit is 2000 chars)
                    if len(analysis) > 2000:
                        for i in range(0, len(analysis), 2000):
                            await channel.send(analysis[i : i + 2000])
                    else:
                        await channel.send(analysis)

                    print(f"✅ Analysis sent for {data['name']}")

                except Exception as ai_error:
                    print(f"❌ AI Error for {data['name']}: {ai_error}")
                    await channel.send(f"⚠️ Lỗi AI: {str(ai_error)[:200]}")

        except Exception as e:
            print(f"❌ Error checking {data.get('name', puuid)}: {e}")
            # Try to notify channel about error
            try:
                channel = bot.get_channel(data.get("channel_id"))
                if channel:
                    await channel.send(
                        f"⚠️ Lỗi khi kiểm tra trận của {data.get('name')}: {str(e)[:100]}"
                    )
            except Exception:
                pass  # Ignore errors when notifying about errors


@check_matches.before_loop
async def before_check_matches():
    await bot.wait_until_ready()


if __name__ == "__main__":
    if not DISCORD_TOKEN:
        print("Error: DISCORD_TOKEN not found in .env file.")
    else:
        print("Starting web server...")
        keep_alive()  # Run fake web server for Render
        print("Starting bot...")
        bot.run(DISCORD_TOKEN)
