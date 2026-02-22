// Package ai provides AI analysis types for ZoeBot.
package ai

// PlayerAnalysis represents AI analysis for a single player.
type PlayerAnalysis struct {
	Champion         string  `json:"champion"`
	PlayerName       string  `json:"player_name"`
	PositionVN       string  `json:"position_vn"`
	Score            float64 `json:"score"`
	VsOpponent       string  `json:"vs_opponent"`
	RoleAnalysis     string  `json:"role_analysis"`
	Highlight        string  `json:"highlight"`
	Weakness         string  `json:"weakness"`
	Comment          string  `json:"comment"`
	TimelineAnalysis string  `json:"timeline_analysis"`
}

// AnalysisResult represents the full AI analysis result.
type AnalysisResult struct {
	Players []PlayerAnalysis `json:"players"`
}

// ChatMessage represents a message in the chat completion request.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the request to the AI API.
type ChatRequest struct {
	Model          string            `json:"model"`
	Messages       []ChatMessage     `json:"messages"`
	Temperature    float64           `json:"temperature"`
	Stream         bool              `json:"stream"`
	MaxTokens      int               `json:"max_tokens"`
	TopP           float64           `json:"top_p"`
	ResponseFormat *ResponseFormat   `json:"response_format,omitempty"`
}

// ResponseFormat specifies the format for AI response.
type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema represents the JSON schema for structured output.
type JSONSchema struct {
	Name   string      `json:"name"`
	Strict bool        `json:"strict"`
	Schema interface{} `json:"schema"`
}

// ChatResponse represents the response from the AI API.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
