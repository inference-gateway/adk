package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// WebSearchSkill implements web search capabilities
type WebSearchSkill struct {
	logger   *zap.Logger
	config   *WebSearchConfig
	security *SecurityValidator
	client   *http.Client
}

// SearchResult represents a single search result
type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

// WebSearchSkillResult represents the result of a web search operation
type WebSearchSkillResult struct {
	Query       string         `json:"query"`
	Engine      string         `json:"engine"`
	Results     []SearchResult `json:"results"`
	ResultCount int            `json:"result_count"`
	SearchTime  string         `json:"search_time"`
}

// NewWebSearchSkill creates a new WebSearch skill
func NewWebSearchSkill(logger *zap.Logger, config *WebSearchConfig, security *SecurityValidator) *WebSearchSkill {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	return &WebSearchSkill{
		logger:   logger,
		config:   config,
		security: security,
		client:   client,
	}
}

// GetName returns the skill name
func (wss *WebSearchSkill) GetName() string {
	return "web_search"
}

// GetDescription returns the skill description
func (wss *WebSearchSkill) GetDescription() string {
	return "Search the web using available search engines. Returns structured results with titles, URLs, and descriptions."
}

// GetParameters returns the JSON schema for skill parameters
func (wss *WebSearchSkill) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query string",
			},
			"engine": map[string]any{
				"type":        "string",
				"description": "Search engine to use (duckduckgo, google, bing). Defaults to duckduckgo",
				"enum":        []string{"duckduckgo", "google", "bing"},
				"default":     "duckduckgo",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (1-50)",
				"minimum":     1,
				"maximum":     50,
				"default":     10,
			},
			"safe_search": map[string]any{
				"type":        "boolean",
				"description": "Enable safe search filtering (default: true)",
				"default":     true,
			},
		},
		"required": []string{"query"},
	}
}

// IsEnabled returns whether the skill is enabled
func (wss *WebSearchSkill) IsEnabled() bool {
	return wss.config.Enabled
}

// GetRateLimit returns the rate limit for this skill
func (wss *WebSearchSkill) GetRateLimit() int {
	return wss.config.RateLimitPerMin
}

// RequiresApproval returns whether this skill requires user approval
func (wss *WebSearchSkill) RequiresApproval() bool {
	return wss.config.RequireApproval
}

// Execute performs the web search operation
func (wss *WebSearchSkill) Execute(ctx context.Context, arguments map[string]any) (string, error) {
	startTime := time.Now()

	// Extract and validate arguments
	queryRaw, ok := arguments["query"]
	if !ok {
		return "", fmt.Errorf("query parameter is required")
	}

	query, ok := queryRaw.(string)
	if !ok {
		return "", fmt.Errorf("query must be a string")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	// Extract optional parameters
	engine := "duckduckgo"
	if val, exists := arguments["engine"]; exists {
		if eng, ok := val.(string); ok {
			engine = strings.ToLower(eng)
		}
	}

	maxResults := wss.config.MaxResults
	if val, exists := arguments["max_results"]; exists {
		if num, ok := val.(float64); ok {
			maxResults = int(num)
		}
	}

	safeSearch := true
	if val, exists := arguments["safe_search"]; exists {
		if safe, ok := val.(bool); ok {
			safeSearch = safe
		}
	}

	// Validate engine is allowed
	if err := wss.validateEngine(engine); err != nil {
		return "", fmt.Errorf("engine validation failed: %w", err)
	}

	// Clamp max results
	if maxResults < 1 {
		maxResults = 1
	}
	if maxResults > wss.config.MaxResults || maxResults > 50 {
		maxResults = wss.config.MaxResults
		if maxResults > 50 {
			maxResults = 50
		}
	}

	// Perform the search
	results, err := wss.performSearch(ctx, query, engine, maxResults, safeSearch)
	if err != nil {
		wss.logger.Error("failed to perform web search", 
			zap.String("query", query),
			zap.String("engine", engine),
			zap.Error(err))
		return "", fmt.Errorf("failed to perform web search: %w", err)
	}

	searchTime := time.Since(startTime).String()

	result := &WebSearchSkillResult{
		Query:       query,
		Engine:      engine,
		Results:     results,
		ResultCount: len(results),
		SearchTime:  searchTime,
	}

	// Return JSON result
	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	wss.logger.Info("successfully performed web search", 
		zap.String("query", query),
		zap.String("engine", engine),
		zap.Int("result_count", len(results)),
		zap.String("search_time", searchTime))

	return string(jsonData), nil
}

// validateEngine checks if the search engine is allowed
func (wss *WebSearchSkill) validateEngine(engine string) error {
	if len(wss.config.AllowedEngines) == 0 {
		// No restrictions, allow common engines
		allowed := []string{"duckduckgo", "google", "bing"}
		for _, allowedEngine := range allowed {
			if engine == allowedEngine {
				return nil
			}
		}
		return fmt.Errorf("unsupported search engine: %s", engine)
	}

	for _, allowedEngine := range wss.config.AllowedEngines {
		if strings.ToLower(allowedEngine) == engine {
			return nil
		}
	}

	return fmt.Errorf("search engine '%s' not in allowed list: %v", engine, wss.config.AllowedEngines)
}

// performSearch executes the actual web search
func (wss *WebSearchSkill) performSearch(ctx context.Context, query, engine string, maxResults int, safeSearch bool) ([]SearchResult, error) {
	switch engine {
	case "duckduckgo":
		return wss.searchDuckDuckGo(ctx, query, maxResults, safeSearch)
	case "google":
		return wss.searchGoogle(ctx, query, maxResults, safeSearch)
	case "bing":
		return wss.searchBing(ctx, query, maxResults, safeSearch)
	default:
		return nil, fmt.Errorf("unsupported search engine: %s", engine)
	}
}

// searchDuckDuckGo performs search using DuckDuckGo's instant answer API
func (wss *WebSearchSkill) searchDuckDuckGo(ctx context.Context, query string, maxResults int, safeSearch bool) ([]SearchResult, error) {
	// DuckDuckGo instant answer API (free, no API key required)
	baseURL := "https://api.duckduckgo.com/"
	
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("no_redirect", "1")
	params.Set("no_html", "1")
	params.Set("skip_disambig", "1")
	
	if safeSearch {
		params.Set("safe_search", "strict")
	}

	searchURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "A2A-Agent/1.0")

	resp, err := wss.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform search request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse DuckDuckGo response
	var ddgResponse map[string]interface{}
	if err := json.Unmarshal(body, &ddgResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var results []SearchResult

	// Extract abstract if available
	if abstract, ok := ddgResponse["Abstract"].(string); ok && abstract != "" {
		if abstractURL, ok := ddgResponse["AbstractURL"].(string); ok && abstractURL != "" {
			result := SearchResult{
				Title:       "Abstract",
				URL:         abstractURL,
				Description: abstract,
				Source:      "duckduckgo",
			}
			results = append(results, result)
		}
	}

	// Extract related topics
	if relatedTopics, ok := ddgResponse["RelatedTopics"].([]interface{}); ok {
		for _, topic := range relatedTopics {
			if len(results) >= maxResults {
				break
			}
			
			if topicMap, ok := topic.(map[string]interface{}); ok {
				title := ""
				url := ""
				description := ""

				if text, ok := topicMap["Text"].(string); ok {
					description = text
					// Extract title from text (usually first part before dash)
					if parts := strings.SplitN(text, " - ", 2); len(parts) > 0 {
						title = parts[0]
					}
				}

				if firstURL, ok := topicMap["FirstURL"].(string); ok {
					url = firstURL
				}

				if title != "" && url != "" {
					result := SearchResult{
						Title:       title,
						URL:         url,
						Description: description,
						Source:      "duckduckgo",
					}
					results = append(results, result)
				}
			}
		}
	}

	return results, nil
}

// searchGoogle performs search using Google Custom Search API (requires API key)
func (wss *WebSearchSkill) searchGoogle(ctx context.Context, query string, maxResults int, safeSearch bool) ([]SearchResult, error) {
	if wss.config.APIKey == "" {
		return nil, fmt.Errorf("google search requires API key")
	}

	// Note: This is a simplified implementation
	// In a real implementation, you would use Google Custom Search API
	// For now, return an error indicating it needs proper setup
	return nil, fmt.Errorf("google search requires proper API setup (Custom Search Engine ID and API key)")
}

// searchBing performs search using Bing Search API (requires API key)  
func (wss *WebSearchSkill) searchBing(ctx context.Context, query string, maxResults int, safeSearch bool) ([]SearchResult, error) {
	if wss.config.APIKey == "" {
		return nil, fmt.Errorf("bing search requires API key")
	}

	// Note: This is a simplified implementation
	// In a real implementation, you would use Bing Search API
	// For now, return an error indicating it needs proper setup
	return nil, fmt.Errorf("bing search requires proper API setup and subscription key")
}

// Validate validates the arguments for the web search operation
func (wss *WebSearchSkill) Validate(arguments map[string]any) error {
	queryRaw, ok := arguments["query"]
	if !ok {
		return fmt.Errorf("query parameter is required")
	}

	query, ok := queryRaw.(string)
	if !ok {
		return fmt.Errorf("query must be a string")
	}

	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("query cannot be empty")
	}

	// Validate optional parameters
	if val, exists := arguments["engine"]; exists {
		engine, ok := val.(string)
		if !ok {
			return fmt.Errorf("engine must be a string")
		}
		
		allowedEngines := []string{"duckduckgo", "google", "bing"}
		valid := false
		for _, allowed := range allowedEngines {
			if strings.ToLower(engine) == allowed {
				valid = true
				break
			}
		}
		
		if !valid {
			return fmt.Errorf("engine must be one of: %v", allowedEngines)
		}
	}

	if val, exists := arguments["max_results"]; exists {
		if num, ok := val.(float64); !ok || num < 1 || num > 50 {
			return fmt.Errorf("max_results must be an integer between 1 and 50")
		}
	}

	if val, exists := arguments["safe_search"]; exists {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("safe_search must be a boolean")
		}
	}

	return nil
}