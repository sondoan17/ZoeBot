import os
import discord
from discord.ext import commands, tasks
from dotenv import load_dotenv
from riot_module import RiotAPI
from ai_module import AIAnalysis
from keep_alive import keep_alive
import asyncio


# Load environment variables
load_dotenv()

DISCORD_TOKEN = os.getenv('DISCORD_TOKEN')
RIOT_API_KEY = os.getenv('RIOT_API_KEY')
GEMINI_API_KEY = os.getenv('GEMINI_API_KEY')

# Bot setup
intents = discord.Intents.default()
intents.message_content = True

bot = commands.Bot(command_prefix='!', intents=intents)
riot_client = RiotAPI(RIOT_API_KEY)
ai_client = AIAnalysis(GEMINI_API_KEY)

# Tracking Data (In-memory for now)
# Format: {puuid: {'last_match_id': str, 'channel_id': int, 'name': str}}
tracked_players = {}

@bot.event
async def on_ready():
    print(f'Bot connected as {bot.user}')
    if not check_matches.is_running():
        check_matches.start()
    print("Polling task started!")

@bot.command()
async def ping(ctx):
    await ctx.send('Pong!')

@bot.command()
async def track(ctx, riot_id: str):
    """
    Track a player. Format: !track Name#Tag
    """
    try:
        if '#' not in riot_id:
            await ctx.send("❌ Sai định dạng! Vui lòng dùng: `Name#Tag` (VD: Faker#SKT)")
            return

        game_name, tag_line = riot_id.split('#', 1)
        await ctx.send(f"🔍 Đang tìm kiếm **{game_name}** #{tag_line}...")

        puuid = riot_client.get_puuid_by_riot_id(game_name, tag_line)
        
        if not puuid:
            await ctx.send("❌ Không tìm thấy người chơi này. Kiểm tra lại tên và tag.")
            return

        # Get latest match to initialize
        matches = riot_client.get_match_ids_by_puuid(puuid, count=1)
        last_match_id = matches[0] if matches else None

        tracked_players[puuid] = {
            'last_match_id': last_match_id,
            'channel_id': ctx.channel.id,
            'name': riot_id
        }

        await ctx.send(f"✅ Đã thêm **{riot_id}** vào danh sách theo dõi!\nBot sẽ thông báo khi có trận mới.")
        print(f"Tracked: {riot_id} (PUUID: {puuid})")

    except Exception as e:
        await ctx.send(f"⚠️ Có lỗi xảy ra: {str(e)}")

@tasks.loop(minutes=2.0)
async def check_matches():
    if not tracked_players:
        return

    print(f"🔄 Checking matches for {len(tracked_players)} players...")
    
    # Iterate copy of items to avoid modification issues during iteration (though here we just modify values)
    for puuid, data in tracked_players.items():
        try:
            matches = riot_client.get_match_ids_by_puuid(puuid, count=1)
            if not matches:
                continue

            latest_match_id = matches[0]
            old_match_id = data['last_match_id']

            # If new match found (and we had a previous record to compare)
            if latest_match_id != old_match_id:
                # Update first to prevent spam if processing fails
                tracked_players[puuid]['last_match_id'] = latest_match_id
                
                if old_match_id is None:
                    # First run/init, just update
                    continue
                
                print(f"🆕 New match found for {data['name']}: {latest_match_id}")
                
                # Fetch details
                channel_id = data['channel_id']
                channel = bot.get_channel(channel_id)
                if channel:
                   await channel.send(f"🚨 **TRẬN MỚI:** {data['name']} vừa chơi xong trận {latest_match_id}!\n⏳ Đang phân tích...")
                   
                   match_details = riot_client.get_match_details(latest_match_id)
                   if match_details:
                       filtered_data = riot_client.parse_match_data(match_details, puuid)
                       if filtered_data:
                           analysis = await ai_client.analyze_match(filtered_data)
                           await channel.send(analysis)
                       else:
                           await channel.send("⚠️ Không thể lấy dữ liệu chi tiết của trận đấu.")

        except Exception as e:
            print(f"Error checking {puuid}: {e}")

@check_matches.before_loop
async def before_check_matches():
    await bot.wait_until_ready()

if __name__ == "__main__":
    if not DISCORD_TOKEN:
        print("Error: DISCORD_TOKEN not found in .env file.")
    else:
        print("Starting web server...")
        keep_alive() # Run fake web server for Render
        print("Starting bot...")
        bot.run(DISCORD_TOKEN)
