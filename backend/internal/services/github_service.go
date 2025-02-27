package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"neobase-ai/pkg/redis"
)

type GitHubService interface {
	GetStarCount(ctx context.Context) (int, error)
}

type githubService struct {
	redisRepo redis.IRedisRepositories
}

type GitHubRepoResponse struct {
	StargazersCount int `json:"stargazers_count"`
}

const (
	starCountKey     = "github:star_count"
	starCountTTL     = 24 * time.Hour
	githubRepoAPIURL = "https://api.github.com/repos/bhaskarblur/neobase-ai-dba"
)

func NewGitHubService(redisRepo redis.IRedisRepositories) GitHubService {
	return &githubService{
		redisRepo: redisRepo,
	}
}

func (s *githubService) GetStarCount(ctx context.Context) (int, error) {
	// Try to get from cache first
	cachedCount, err := s.redisRepo.Get(starCountKey, ctx)
	if err == nil && cachedCount != "" {
		var count int
		if err := json.Unmarshal([]byte(cachedCount), &count); err == nil {
			log.Printf("GitHub star count fetched from cache: %d", count)
			return count, nil
		}
	}

	// Cache miss or error, fetch from GitHub
	count, err := s.fetchStarCount()
	if err != nil {
		log.Printf("Error fetching star count from GitHub: %v", err)
		return 0, err
	}

	// Store in cache
	countBytes, _ := json.Marshal(count)
	if err := s.redisRepo.Set(starCountKey, countBytes, starCountTTL, ctx); err != nil {
		log.Printf("Error caching star count: %v", err)
	} else {
		log.Printf("GitHub star count cached successfully: %d", count)
	}

	return count, nil
}

func (s *githubService) fetchStarCount() (int, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", githubRepoAPIURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %v", err)
	}

	// Add User-Agent header to avoid GitHub API rate limiting
	req.Header.Set("User-Agent", "NeoBase-AI-DBA")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch from GitHub API: %v", err)
		return 0, fmt.Errorf("failed to fetch from GitHub API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("GitHub API returned status: %d", resp.StatusCode)
		return 0, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var repoData GitHubRepoResponse
	if err := json.NewDecoder(resp.Body).Decode(&repoData); err != nil {
		return 0, fmt.Errorf("failed to decode GitHub response: %v", err)
	}

	log.Printf("GitHub star count fetched from API: %d", repoData.StargazersCount)
	return repoData.StargazersCount, nil
}
