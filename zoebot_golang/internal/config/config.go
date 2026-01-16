// Package config provides configuration management for ZoeBot.
package config

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config holds all configuration values for the application.
type Config struct {
	// Discord
	DiscordToken string

	// Riot API
	RiotAPIKey         string
	RiotBaseURLAccount string
	RiotBaseURLMatch   string
	RiotBaseURLPlatform string // For summoner/league APIs (vn2.api.riotgames.com)

	// AI / LLM API
	AIAPIKey string
	AIAPIURL string
	AIModel  string

	// Redis
	RedisURL               string
	RedisKeyTrackedPlayers string

	// Data Dragon
	DDragonVersion        string
	DDragonChampionIconURL string

	// Paths
	DataDir string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	// Try to load .env file (ignore error if not exists)
	_ = godotenv.Load()

	cfg := &Config{
		// Discord
		DiscordToken: os.Getenv("DISCORD_TOKEN"),

		// Riot API
		RiotAPIKey:          os.Getenv("RIOT_API_KEY"),
		RiotBaseURLAccount:  getEnvOrDefault("RIOT_BASE_URL_ACCOUNT", "https://asia.api.riotgames.com"),
		RiotBaseURLMatch:    getEnvOrDefault("RIOT_BASE_URL_MATCH", "https://sea.api.riotgames.com"),
		RiotBaseURLPlatform: getEnvOrDefault("RIOT_BASE_URL_PLATFORM", "https://vn2.api.riotgames.com"),

		// AI / LLM API
		AIAPIKey: os.Getenv("CLIPROXY_API_KEY"),
		AIAPIURL: os.Getenv("CLIPROXY_API_URL"),
		AIModel:  os.Getenv("CLIPROXY_MODEL"),

		// Redis
		RedisURL:               os.Getenv("REDIS_URL"),
		RedisKeyTrackedPlayers: getEnvOrDefault("REDIS_KEY_TRACKED_PLAYERS", "zoebot:tracked_players"),

		// Data Dragon
		DDragonVersion:         getEnvOrDefault("DDRAGON_VERSION", "16.1.1"),
		DDragonChampionIconURL: "", // Will be set below

		// Paths
		DataDir: getEnvOrDefault("DATA_DIR", "data"),
	}

	// Build champion icon URL template
	cfg.DDragonChampionIconURL = "https://ddragon.leagueoflegends.com/cdn/" + cfg.DDragonVersion + "/img/champion/%s.png"

	return cfg, nil
}

// Validate checks if all required configuration values are set.
func (c *Config) Validate() error {
	var errs []string

	if c.DiscordToken == "" {
		errs = append(errs, "DISCORD_TOKEN is missing")
	}

	if c.RiotAPIKey == "" {
		errs = append(errs, "RIOT_API_KEY is missing")
	}

	if c.AIAPIKey == "" {
		errs = append(errs, "CLIPROXY_API_KEY is missing")
	}

	if len(errs) > 0 {
		log.Println("Config errors:")
		for _, e := range errs {
			log.Printf("  - %s", e)
		}
		return errors.New("configuration validation failed")
	}

	return nil
}

// ChampionDataPath returns the full path to champion.json
func (c *Config) ChampionDataPath() string {
	return filepath.Join(c.DataDir, "champion.json")
}

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
