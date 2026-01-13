import requests
import json
import logging
import asyncio

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class AIAnalysis:
    def __init__(self, api_key):
        self.api_key = api_key
        self.api_url = "https://openrouter.ai/api/v1/chat/completions"
        self.model = "tngtech/deepseek-r1t2-chimera:free" # Default free model, can be changed
        
        if not api_key:
            logger.error("OpenRouter API Key is missing!")

    async def analyze_match(self, match_data):
        """
        Sends match data to OpenRouter to generate a coach-like analysis.
        """
        if not self.api_key:
             return "⚠️ Lỗi: Chưa cấu hình OpenRouter API Key."

        if not match_data:
            return "Error: No match data provided."

        target_player = match_data.get('target_player')
        if not target_player:
            return "Error: Target player data missing."

        # Construct the prompt
        prompt = f"""
        Bạn là một huấn luyện viên Liên Minh Huyền Thoại (League of Legends) chuyên nghiệp, vui tính và khắt khe.
        Hãy phân tích trận đấu của người chơi: {target_player.get('riotIdGameName')} (Champion: {target_player.get('championName')}) và toàn bộ đồng đội trong team của họ.

        **Thông tin trận đấu:**
        - Chế độ: {match_data.get('gameMode')}
        - Thời lượng: {match_data.get('gameDuration')} giây
        - ID trận: {match_data.get('matchId')}
        
        **Dữ liệu chi tiết các người chơi:**
        {json.dumps(match_data.get('all_players'), indent=2)}

        **Yêu cầu phân tích:**
        Hãy xác định team của người chơi mục tiêu ({target_player.get('riotIdGameName')}) và **chỉ phân tích 5 thành viên trong team đó**.
        
        **Định dạng Output (Bắt buộc tuân thủ):**
        Với mỗi thành viên trong team, hãy xuất ra theo định dạng sau (không dùng markdown table, dùng list hoặc block):

        [Tên Tướng] [Tên người chơi] ([Vị trí Tiếng Việt]) - [Điểm số]/10 - [Lời bình về KDA, sát thương, farm, đóng góp... tối đa 2 câu].

        ---
        **Ví dụ mẫu:**
        Zeri - Arsene Lupin - Xạ thủ - 7.5/10 - Người gây sát thương mạnh nhất trận đấu (47,240) và chỉ số lính ấn tượng (378). Zeri đã nỗ lực hết mình nhưng không thể bù đắp được khoảng trống của đồng đội.

        ... (Lặp lại cho đủ 5 thành viên)

        **Lưu ý:**
        - Vị trí Tiếng Việt: Đường trên, Đi rừng, Đường giữa, Xạ thủ, Hỗ trợ.
        - Sắp xếp theo thứ tự lane nếu có thể.
        - Đánh giá khách quan dựa trên stats.
        """
        
        payload = {
            "model": self.model,
            "messages": [
                {
                    "role": "user",
                    "content": prompt
                }
            ],
            "reasoning": {"enabled": True}
        }
        
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "HTTP-Referer": "https://github.com/sondoan17/ZoeBot", # Optional
            "X-Title": "ZoeBot", # Optional
            "Content-Type": "application/json"
        }

        try:
            # Run blocking call in a separate thread
            response = await asyncio.to_thread(
                requests.post,
                url=self.api_url,
                headers=headers,
                data=json.dumps(payload)
            )
            
            if response.status_code == 200:
                result = response.json()
                return result['choices'][0]['message']['content']
            else:
                logger.error(f"OpenRouter Error: {response.status_code} - {response.text}")
                return f"⚠️ Lỗi OpenRouter ({response.status_code}): {response.text}"
                
        except Exception as e:
            logger.error(f"AI Generation Error: {e}")
            return f"⚠️ Lỗi hệ thống AI: {str(e)}"
