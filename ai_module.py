from google import genai
import json
import logging
import asyncio

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class AIAnalysis:
    def __init__(self, api_key):
        self.client = None
        if not api_key:
            logger.error("Gemini API Key is missing!")
            return
        
        self.client = genai.Client(api_key=api_key)

    async def analyze_match(self, match_data):
        """
        Sends match data to Gemini to generate a coach-like analysis.
        """
        if not self.client:
             return "⚠️ Lỗi: Chưa cấu hình Gemini API Key."

        if not match_data:
            return "Error: No match data provided."

        target_player = match_data.get('target_player')
        if not target_player:
            return "Error: Target player data missing."

        # Construct the prompt
        prompt = f"""
        Bạn là một huấn luyện viên Liên Minh Huyền Thoại (League of Legends) chuyên nghiệp, vui tính và khắt khe.
        Hãy phân tích dữ liệu trận đấu dưới đây cho người chơi: {target_player.get('riotIdGameName')} (Champion: {target_player.get('championName')}, Lane: {target_player.get('teamPosition')}).

        **Thông tin trận đấu:**
        - Chế độ: {match_data.get('gameMode')}
        - Thời lượng: {match_data.get('gameDuration')} giây
        - ID trận: {match_data.get('matchId')}
        
        **Dữ liệu người chơi:**
        {json.dumps(target_player, indent=2)}

        **Yêu cầu phân tích:**
        1. **Tóm tắt nhanh:** Trận đấu diễn ra thế nào? (Thắng/Thua, KDA có tốt không?).
        2. **Đánh giá chi tiết:**
           - Khả năng farm (CS).
           - Đóng góp sát thương (Damage Dealt).
           - Khả năng sống sót (Deaths, Damage Taken).
           - Tầm nhìn (Vision Score).
           - Cách lên đồ (Items): Phân tích xem build đồ có hợp lý không.
        3. **Lời khuyên:** Đưa ra 2-3 lời khuyên cụ thể để cải thiện.
        4. **Chấm điểm:** Chấm điểm màn trình diễn trên thang 10.

        **Output format:** Trả về định dạng Markdown đẹp, dễ đọc. Dùng emoji phù hợp.
        """

        try:
            # Run blocking call in a separate thread
            response = await asyncio.to_thread(
                self.client.models.generate_content,
                model="gemini-3-flash",
                contents=prompt
            )
            return response.text
        except Exception as e:
            logger.error(f"AI Generation Error: {e}")
            return f"⚠️ Lỗi hệ thống AI: {str(e)}"
