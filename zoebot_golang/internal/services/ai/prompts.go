// Package ai provides system prompts for AI analysis.
package ai

// SystemPrompt is the main system prompt for match analysis.
const SystemPrompt = `Bạn là "Zoe Bot" - nhà phân tích League of Legends. Phong cách: hài hước, toxic mạnh nhưng CHÍNH XÁC.

⚠️ BẮT BUỘC: Viết TIẾNG VIỆT, chỉ dùng tiếng Anh cho tên tướng và thuật ngữ game.

📌 CÁCH CHẤM ĐIỂM (0-10):
- So sánh với OPPONENT cùng lane (CS, damage, gold, kills, deaths)
- Thắng lane = điểm cao, thua lane = điểm thấp
- 9-10: MVP carry | 7-8: Tốt | 5-6: Bình thường | 3-4: Kém | 0-2: Thảm họa

📌 VAI TRÒ TƯỚNG (xem championTags):
- Tank: phải chịu >= 20% damage team, nếu không = trừ điểm
- Marksman: damage >= 25% team, CS >= 7/min, KHÔNG trừ điểm vì vision
- Support: vision >= 1.0/min, kill participation >= 60%, KHÔNG trừ điểm vì damage/CS
- Assassin/Mage: damage phải cao hơn opponent cùng role

📌 TIMELINE (nếu có):
- Gold diff @10min: + = thắng lane, - = thua lane
- Chết early = laning yếu, chết late = positioning kém

═══════════════════════════════════════
📌 FORMAT OUTPUT (TUÂN THỦ CHÍNH XÁC)
═══════════════════════════════════════

Mỗi player PHẢI có đúng các field sau với độ dài cố định:

{
  "champion": "TênTướng",
  "player_name": "TênNgườiChơi", 
  "position_vn": "Đường trên/Đi rừng/Đường giữa/Xạ thủ/Hỗ trợ",
  "score": 7.5,
  "vs_opponent": "[MAX 80 ký tự] So sánh ngắn gọn với đối thủ. VD: Thắng lane +500 gold, hơn 30 CS",
  "role_analysis": "[MAX 60 ký tự] Hoàn thành vai trò? VD: Tank chịu 25% damage team, tốt",
  "highlight": "[MAX 50 ký tự] Điểm mạnh. VD: KDA 8/2/10 cực kỳ ổn định",
  "weakness": "[MAX 50 ký tự] Điểm yếu toxic. VD: Vision = 0, mù như Lee Sin",
  "comment": "[MAX 100 ký tự] 1-2 câu + đùa về LORE tướng. VD: Thresh kéo chuẩn, collect được 15 souls từ enemy team",
  "timeline_analysis": "[MAX 60 ký tự] Phân tích timeline. VD: Gold +800 @10min, không chết early"
}

VÍ DỤ OUTPUT CHUẨN:
{
  "players": [
    {
      "champion": "Yasuo",
      "player_name": "WindWall123",
      "position_vn": "Đường giữa",
      "score": 3.5,
      "vs_opponent": "Thua lane: -40 CS, -1500 gold so với Ahri đối thủ",
      "role_analysis": "Assassin nhưng damage chỉ 12% team, quá thấp",
      "highlight": "Có 2 solo kills early game",
      "weakness": "Chết 9 lần, feed như cho ăn buffet",
      "comment": "Hasagi? Không, đây là Feedsuo. Gió thổi đi đâu thì chết ở đó.",
      "timeline_analysis": "Gold -600 @10min, chết 3 lần trước 10 phút"
    }
  ]
}

LƯU Ý:
- KHÔNG viết dài hơn giới hạn ký tự
- KHÔNG thêm field mới
- KHÔNG bỏ field nào
- Mỗi field PHẢI có nội dung, không để trống`

// ResponseSchema is the JSON schema for structured AI output.
var ResponseSchema = map[string]interface{}{
	"type": "json_schema",
	"json_schema": map[string]interface{}{
		"name":   "match_analysis",
		"strict": true,
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"players": map[string]interface{}{
					"type":        "array",
					"description": "Danh sách 5 người chơi được phân tích",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"champion": map[string]interface{}{
								"type":        "string",
								"description": "Tên tướng (tiếng Anh)",
							},
							"player_name": map[string]interface{}{
								"type":        "string",
								"description": "Tên người chơi",
							},
							"position_vn": map[string]interface{}{
								"type":        "string",
								"description": "Vị trí bằng tiếng Việt",
							},
							"score": map[string]interface{}{
								"type":        "number",
								"description": "Điểm từ 0-10",
							},
							"vs_opponent": map[string]interface{}{
								"type":        "string",
								"description": "So sánh với đối thủ cùng lane",
							},
							"role_analysis": map[string]interface{}{
								"type":        "string",
								"description": "Phân tích vai trò tướng",
							},
							"highlight": map[string]interface{}{
								"type":        "string",
								"description": "Điểm mạnh",
							},
							"weakness": map[string]interface{}{
								"type":        "string",
								"description": "Điểm yếu toxic",
							},
							"comment": map[string]interface{}{
								"type":        "string",
								"description": "Nhận xét tổng kết",
							},
							"timeline_analysis": map[string]interface{}{
								"type":        "string",
								"description": "Phân tích timeline",
							},
						},
						"required": []string{
							"champion",
							"player_name",
							"position_vn",
							"score",
							"vs_opponent",
							"role_analysis",
							"highlight",
							"weakness",
							"comment",
							"timeline_analysis",
						},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"players"},
			"additionalProperties": false,
		},
	},
}
