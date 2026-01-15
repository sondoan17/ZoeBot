// Package ai provides AI analysis client for ZoeBot.
package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zoebot/internal/config"
	"github.com/zoebot/internal/services/riot"
)

// Client is a client for AI analysis API.
type Client struct {
	apiKey     string
	apiURL     string
	model      string
	httpClient *http.Client
}

// NewClient creates a new AI client.
func NewClient(cfg *config.Config) *Client {
	c := &Client{
		apiKey: cfg.AIAPIKey,
		apiURL: cfg.AIAPIURL,
		model:  cfg.AIModel,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}

	if c.apiKey != "" {
		log.Printf("✅ Loaded AI API Key: %s*** (length: %d)", c.apiKey[:4], len(c.apiKey))
	} else {
		log.Println("⚠️ AI API Key is missing!")
	}

	log.Printf("📡 AI API URL: %s", c.apiURL)
	log.Printf("🤖 AI Model: %s", c.model)

	return c
}

// AnalyzeMatch analyzes match data and returns structured result.
func (c *Client) AnalyzeMatch(matchData *riot.ParsedMatchData) (*AnalysisResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}

	if matchData == nil || len(matchData.Teammates) == 0 {
		return nil, fmt.Errorf("invalid match data")
	}

	content, err := c.makeAPIRequest(matchData)
	if err != nil {
		return nil, err
	}

	return c.parseResponse(content)
}

// buildUserPrompt builds the user prompt from match data.
func (c *Client) buildUserPrompt(matchData *riot.ParsedMatchData) string {
	var sb strings.Builder

	winText := "💀 THUA"
	if matchData.Win {
		winText = "🏆 THẮNG"
	}

	sb.WriteString("THÔNG TIN TRẬN ĐẤU:\n")
	sb.WriteString(fmt.Sprintf("- Chế độ: %s\n", matchData.GameMode))
	sb.WriteString(fmt.Sprintf("- Thời lượng: %.1f phút\n", matchData.GameDurationMinutes))
	sb.WriteString(fmt.Sprintf("- Kết quả: %s\n", winText))
	sb.WriteString(fmt.Sprintf("- Người chơi chính: %s\n\n", matchData.TargetPlayerName))

	// Lane matchups as JSON
	matchupsJSON, _ := json.MarshalIndent(matchData.LaneMatchups, "", "  ")
	sb.WriteString("SO SÁNH TỪNG LANE (Player vs Opponent):\n")
	sb.WriteString(string(matchupsJSON))

	// Timeline insights
	if matchData.TimelineInsights != nil {
		sb.WriteString(c.buildTimelineText(matchData.TimelineInsights))
	}

	sb.WriteString("\n\nPhân tích 5 người chơi. So sánh với đối thủ cùng lane, kiểm tra vai trò tướng, và xem xét timeline data nếu có.")

	return sb.String()
}

// buildTimelineText builds formatted timeline text.
func (c *Client) buildTimelineText(timeline *riot.TimelineData) string {
	var sb strings.Builder

	sb.WriteString("\n\nDIỄN BIẾN TRẬN ĐẤU (Timeline):\n")

	// First blood
	if timeline.FirstBlood != nil {
		fb := timeline.FirstBlood
		sb.WriteString(fmt.Sprintf("🩸 First Blood: %s giết %s lúc %.1f phút\n", fb.Killer, fb.Victim, fb.TimeMin))
	} else {
		sb.WriteString("🩸 First Blood: Không có data\n")
	}

	// Gold diff at 10min
	sb.WriteString("💰 Gold Diff @10min vs Lane Opponent:\n")
	if len(timeline.GoldDiff10Min) > 0 {
		for name, data := range timeline.GoldDiff10Min {
			sb.WriteString(fmt.Sprintf("  • %s: %+d gold (%s)\n", name, data.Diff, data.Position))
		}
	} else {
		sb.WriteString("  Không có data\n")
	}

	// Deaths timeline
	sb.WriteString("💀 Deaths Timeline (5 đầu tiên của team):\n")
	if len(timeline.DeathsTimeline) > 0 {
		limit := 5
		if len(timeline.DeathsTimeline) < limit {
			limit = len(timeline.DeathsTimeline)
		}
		for i := 0; i < limit; i++ {
			d := timeline.DeathsTimeline[i]
			sb.WriteString(fmt.Sprintf("  • %s chết lúc %.1f phút bởi %s\n", d.Player, d.TimeMin, d.Killer))
		}
	} else {
		sb.WriteString("  Không có deaths\n")
	}

	// Objectives
	sb.WriteString("🐉 Objectives:\n")
	if len(timeline.ObjectiveKills) > 0 {
		limit := 5
		if len(timeline.ObjectiveKills) < limit {
			limit = len(timeline.ObjectiveKills)
		}
		for i := 0; i < limit; i++ {
			o := timeline.ObjectiveKills[i]
			sb.WriteString(fmt.Sprintf("  • %s lúc %.1f phút bởi %s\n", o.MonsterType, o.TimeMin, o.Killer))
		}
	} else {
		sb.WriteString("  Không có objectives\n")
	}

	// Turret plates
	sb.WriteString(fmt.Sprintf("🏰 Turret Plates: Team lấy %d, mất %d\n",
		timeline.TurretPlatesDestroyed, timeline.TurretPlatesLost))

	return sb.String()
}

// makeAPIRequest makes the API request to the AI service.
func (c *Client) makeAPIRequest(matchData *riot.ParsedMatchData) (string, error) {
	userPrompt := c.buildUserPrompt(matchData)

	payload := ChatRequest{
		Model: c.model,
		Messages: []ChatMessage{
			{Role: "system", Content: SystemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.7,
		MaxTokens:   20000,
		TopP:        1,
		ResponseFormat: &ResponseFormat{
			Type:       "json_schema",
			JSONSchema: &JSONSchema{
				Name:   "match_analysis",
				Strict: true,
				Schema: ResponseSchema["json_schema"].(map[string]interface{})["schema"],
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("AI API Error: %d - %s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// parseResponse parses the AI response content.
func (c *Client) parseResponse(content string) (*AnalysisResult, error) {
	// Clean up markdown code blocks if present
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = content[7:]
	}
	if strings.HasPrefix(content, "```") {
		content = content[3:]
	}
	if strings.HasSuffix(content, "```") {
		content = content[:len(content)-3]
	}
	content = strings.TrimSpace(content)

	var result AnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		log.Printf("Failed to parse AI JSON: %v", err)
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return &result, nil
}

// GetScoreEmoji returns emoji based on player score.
func GetScoreEmoji(score float64) string {
	switch {
	case score >= 8:
		return "🌟"
	case score >= 6:
		return "✅"
	case score >= 4:
		return "⚠️"
	default:
		return "❌"
	}
}
