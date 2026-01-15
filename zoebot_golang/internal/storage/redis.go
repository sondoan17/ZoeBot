// Package storage provides Redis persistence for ZoeBot.
package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/zoebot/internal/config"
)

// TrackedPlayer represents a player being tracked for match notifications.
type TrackedPlayer struct {
	PUUID       string `json:"puuid"`
	LastMatchID string `json:"last_match_id"`
	ChannelID   string `json:"channel_id"`
	Name        string `json:"name"`
}

// RedisClient is a client for Upstash Redis REST API.
type RedisClient struct {
	url     string
	token   string
	enabled bool
	client  *http.Client
	mu      sync.RWMutex
}

// NewRedisClient creates a new Redis client.
// Optimized: shorter timeout, connection reuse
func NewRedisClient(cfg *config.Config) *RedisClient {
	enabled := cfg.UpstashRedisRESTURL != "" && cfg.UpstashRedisRESTToken != ""

	if !enabled {
		log.Println("Redis not configured, using memory")
	}

	transport := &http.Transport{
		MaxIdleConns:        3,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     60 * time.Second,
	}

	return &RedisClient{
		url:     cfg.UpstashRedisRESTURL,
		token:   cfg.UpstashRedisRESTToken,
		enabled: enabled,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport,
		},
	}
}

// redisResponse represents the response from Upstash REST API.
type redisResponse struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

// request makes a request to Upstash Redis REST API.
func (r *RedisClient) request(command []interface{}) (*redisResponse, error) {
	if !r.enabled {
		return nil, nil
	}

	body, err := json.Marshal(command)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	req, err := http.NewRequest("POST", r.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("redis error: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var result redisResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Get retrieves a value from Redis.
func (r *RedisClient) Get(key string) (string, error) {
	result, err := r.request([]interface{}{"GET", key})
	if err != nil {
		return "", err
	}
	if result == nil || result.Result == nil {
		return "", nil
	}

	str, ok := result.Result.(string)
	if !ok {
		return "", nil
	}
	return str, nil
}

// Set stores a value in Redis.
func (r *RedisClient) Set(key string, value string) error {
	result, err := r.request([]interface{}{"SET", key, value})
	if err != nil {
		return err
	}
	if result == nil {
		return nil
	}
	if result.Result != "OK" {
		return fmt.Errorf("set failed: %v", result.Result)
	}
	return nil
}

// Delete removes a key from Redis.
func (r *RedisClient) Delete(key string) error {
	_, err := r.request([]interface{}{"DEL", key})
	return err
}

// TrackedPlayersStore manages tracked players persistence.
type TrackedPlayersStore struct {
	redis   *RedisClient
	key     string
	players map[string]*TrackedPlayer
	mu      sync.RWMutex
}

// NewTrackedPlayersStore creates a new tracked players store.
func NewTrackedPlayersStore(redis *RedisClient, key string) *TrackedPlayersStore {
	return &TrackedPlayersStore{
		redis:   redis,
		key:     key,
		players: make(map[string]*TrackedPlayer),
	}
}

// Load loads tracked players from Redis.
func (s *TrackedPlayersStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.redis.Get(s.key)
	if err != nil {
		return err
	}

	if data == "" {
		s.players = make(map[string]*TrackedPlayer)
		return nil
	}

	var players map[string]*TrackedPlayer
	if err := json.Unmarshal([]byte(data), &players); err != nil {
		return err
	}

	s.players = players
	log.Printf("Loaded %d players", len(s.players))
	return nil
}

// Save saves tracked players to Redis.
func (s *TrackedPlayersStore) Save() error {
	s.mu.RLock()
	data, err := json.Marshal(s.players)
	s.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to marshal players: %w", err)
	}

	if err := s.redis.Set(s.key, string(data)); err != nil {
		return err
	}

	return nil
}

// Get returns a tracked player by PUUID.
func (s *TrackedPlayersStore) Get(puuid string) (*TrackedPlayer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.players[puuid]
	return p, ok
}

// Set adds or updates a tracked player.
func (s *TrackedPlayersStore) Set(puuid string, player *TrackedPlayer) {
	s.mu.Lock()
	s.players[puuid] = player
	s.mu.Unlock()
}

// Delete removes a tracked player.
func (s *TrackedPlayersStore) Delete(puuid string) {
	s.mu.Lock()
	delete(s.players, puuid)
	s.mu.Unlock()
}

// GetAll returns all tracked players.
func (s *TrackedPlayersStore) GetAll() map[string]*TrackedPlayer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*TrackedPlayer, len(s.players))
	for k, v := range s.players {
		result[k] = v
	}
	return result
}

// GetByChannel returns all tracked players for a specific channel.
func (s *TrackedPlayersStore) GetByChannel(channelID string) []*TrackedPlayer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*TrackedPlayer
	for _, p := range s.players {
		if p.ChannelID == channelID {
			result = append(result, p)
		}
	}
	return result
}

// Count returns the number of tracked players.
func (s *TrackedPlayersStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.players)
}

// UpdateLastMatch updates the last match ID for a player.
func (s *TrackedPlayersStore) UpdateLastMatch(puuid, matchID string) {
	s.mu.Lock()
	if p, ok := s.players[puuid]; ok {
		p.LastMatchID = matchID
	}
	s.mu.Unlock()
}
