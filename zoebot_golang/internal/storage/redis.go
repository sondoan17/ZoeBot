// Package storage provides Redis persistence for ZoeBot.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zoebot/internal/config"
)

// TrackedPlayer represents a player being tracked for match notifications.
type TrackedPlayer struct {
	PUUID       string `json:"puuid"`
	LastMatchID string `json:"last_match_id"`
	ChannelID   string `json:"channel_id"`
	Name        string `json:"name"`
}

// RedisClient wraps go-redis client.
type RedisClient struct {
	client  *redis.Client
	enabled bool
	ctx     context.Context
}

// NewRedisClient creates a new Redis client using go-redis.
func NewRedisClient(cfg *config.Config) *RedisClient {
	redisURL := cfg.RedisURL
	if redisURL == "" {
		log.Println("Redis not configured (REDIS_URL missing), using memory only")
		return &RedisClient{enabled: false, ctx: context.Background()}
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("Failed to parse REDIS_URL: %v", err)
		return &RedisClient{enabled: false, ctx: context.Background()}
	}

	// Optimize for serverless
	opt.PoolSize = 5
	opt.MinIdleConns = 1
	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opt)
	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Redis connection failed: %v", err)
		return &RedisClient{enabled: false, ctx: ctx}
	}

	log.Println("Redis connected successfully")
	return &RedisClient{
		client:  client,
		enabled: true,
		ctx:     ctx,
	}
}

// Get retrieves a value from Redis.
func (r *RedisClient) Get(key string) (string, error) {
	if !r.enabled {
		return "", nil
	}
	val, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Set stores a value in Redis (no expiration).
func (r *RedisClient) Set(key string, value string) error {
	if !r.enabled {
		return nil
	}
	return r.client.Set(r.ctx, key, value, 0).Err()
}

// Delete removes a key from Redis.
func (r *RedisClient) Delete(key string) error {
	if !r.enabled {
		return nil
	}
	return r.client.Del(r.ctx, key).Err()
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
