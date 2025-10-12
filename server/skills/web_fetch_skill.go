package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// WebFetchSkill implements web content fetching capabilities
type WebFetchSkill struct {
	logger   *zap.Logger
	config   *WebFetchConfig
	security *SecurityValidator
	client   *http.Client
	cache    *fetchCache
}

// WebFetchSkillResult represents the result of a web fetch operation
type WebFetchSkillResult struct {
	URL           string            `json:"url"`
	Content       string            `json:"content"`
	ContentType   string            `json:"content_type"`
	StatusCode    int               `json:"status_code"`
	ContentLength int64             `json:"content_length"`
	Headers       map[string]string `json:"headers"`
	FetchTime     string            `json:"fetch_time"`
	Cached        bool              `json:"cached"`
	Truncated     bool              `json:"truncated,omitempty"`
}

// cacheEntry represents a cached web fetch result
type cacheEntry struct {
	result    *WebFetchSkillResult
	timestamp time.Time
}

// fetchCache implements a simple in-memory cache for web fetch results
type fetchCache struct {
	entries map[string]*cacheEntry
	mutex   sync.RWMutex
	ttl     time.Duration
}

// NewWebFetchSkill creates a new WebFetch skill
func NewWebFetchSkill(logger *zap.Logger, config *WebFetchConfig, security *SecurityValidator) *WebFetchSkill {
	client := &http.Client{
		Timeout: config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !config.FollowRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	cache := &fetchCache{
		entries: make(map[string]*cacheEntry),
		ttl:     config.CacheTTL,
	}

	return &WebFetchSkill{
		logger:   logger,
		config:   config,
		security: security,
		client:   client,
		cache:    cache,
	}
}

// GetName returns the skill name
func (wfs *WebFetchSkill) GetName() string {
	return "web_fetch"
}

// GetDescription returns the skill description
func (wfs *WebFetchSkill) GetDescription() string {
	return "Fetch content from web URLs with domain whitelisting and content size limits. Supports caching and various content types."
}

// GetParameters returns the JSON schema for skill parameters
func (wfs *WebFetchSkill) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to fetch content from (must be HTTP or HTTPS)",
			},
			"method": map[string]any{
				"type":        "string",
				"description": "HTTP method to use (GET, POST, PUT, DELETE)",
				"enum":        []string{"GET", "POST", "PUT", "DELETE", "HEAD"},
				"default":     "GET",
			},
			"headers": map[string]any{
				"type":        "object",
				"description": "Custom headers to include in the request",
				"additionalProperties": map[string]any{
					"type": "string",
				},
			},
			"body": map[string]any{
				"type":        "string",
				"description": "Request body for POST/PUT requests",
			},
			"follow_redirects": map[string]any{
				"type":        "boolean",
				"description": "Whether to follow HTTP redirects (default: true)",
				"default":     true,
			},
			"use_cache": map[string]any{
				"type":        "boolean",
				"description": "Whether to use cached results if available (default: true)",
				"default":     true,
			},
		},
		"required": []string{"url"},
	}
}

// IsEnabled returns whether the skill is enabled
func (wfs *WebFetchSkill) IsEnabled() bool {
	return wfs.config.Enabled
}

// GetRateLimit returns the rate limit for this skill
func (wfs *WebFetchSkill) GetRateLimit() int {
	return wfs.config.RateLimitPerMin
}

// RequiresApproval returns whether this skill requires user approval
func (wfs *WebFetchSkill) RequiresApproval() bool {
	return wfs.config.RequireApproval
}

// Execute performs the web fetch operation
func (wfs *WebFetchSkill) Execute(ctx context.Context, arguments map[string]any) (string, error) {
	startTime := time.Now()

	// Extract and validate arguments
	urlRaw, ok := arguments["url"]
	if !ok {
		return "", fmt.Errorf("url parameter is required")
	}

	urlStr, ok := urlRaw.(string)
	if !ok {
		return "", fmt.Errorf("url must be a string")
	}

	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return "", fmt.Errorf("url cannot be empty")
	}

	// Validate URL security
	if err := wfs.security.ValidateURL(urlStr, wfs.config.WhitelistedDomains); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
	}

	// Extract optional parameters
	method := "GET"
	if val, exists := arguments["method"]; exists {
		if m, ok := val.(string); ok {
			method = strings.ToUpper(m)
		}
	}

	var customHeaders map[string]string
	if val, exists := arguments["headers"]; exists {
		if headers, ok := val.(map[string]interface{}); ok {
			customHeaders = make(map[string]string)
			for k, v := range headers {
				if strVal, ok := v.(string); ok {
					customHeaders[k] = strVal
				}
			}
		}
	}

	body := ""
	if val, exists := arguments["body"]; exists {
		if b, ok := val.(string); ok {
			body = b
		}
	}

	followRedirects := wfs.config.FollowRedirects
	if val, exists := arguments["follow_redirects"]; exists {
		if follow, ok := val.(bool); ok {
			followRedirects = follow
		}
	}

	useCache := wfs.config.CacheEnabled
	if val, exists := arguments["use_cache"]; exists {
		if cache, ok := val.(bool); ok {
			useCache = cache
		}
	}

	// Check cache first if enabled
	if useCache && method == "GET" && body == "" {
		if cached := wfs.getFromCache(urlStr); cached != nil {
			cached.Cached = true
			cached.FetchTime = time.Since(startTime).String()
			
			jsonData, err := json.Marshal(cached)
			if err != nil {
				return "", fmt.Errorf("failed to marshal cached result: %w", err)
			}
			
			wfs.logger.Info("returned cached web fetch result", 
				zap.String("url", urlStr))
			
			return string(jsonData), nil
		}
	}

	// Perform the fetch
	result, err := wfs.performFetch(ctx, urlStr, method, customHeaders, body, followRedirects)
	if err != nil {
		wfs.logger.Error("failed to fetch web content", 
			zap.String("url", urlStr),
			zap.String("method", method),
			zap.Error(err))
		return "", fmt.Errorf("failed to fetch web content: %w", err)
	}

	result.FetchTime = time.Since(startTime).String()

	// Cache the result if enabled and it's a successful GET request
	if useCache && method == "GET" && body == "" && result.StatusCode == 200 {
		wfs.putInCache(urlStr, result)
	}

	// Return JSON result
	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	wfs.logger.Info("successfully fetched web content", 
		zap.String("url", urlStr),
		zap.String("method", method),
		zap.Int("status_code", result.StatusCode),
		zap.Int64("content_length", result.ContentLength),
		zap.String("fetch_time", result.FetchTime))

	return string(jsonData), nil
}

// performFetch executes the actual web fetch
func (wfs *WebFetchSkill) performFetch(ctx context.Context, urlStr, method string, customHeaders map[string]string, body string, followRedirects bool) (*WebFetchSkillResult, error) {
	// Create request
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("User-Agent", wfs.config.UserAgent)

	// Set custom headers
	for key, value := range customHeaders {
		req.Header.Set(key, value)
	}

	// Configure client redirect behavior for this request
	if !followRedirects {
		// Create a new client with redirect disabled for this request
		client := &http.Client{
			Timeout: wfs.config.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		wfs.client = client
	}

	// Perform the request
	resp, err := wfs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Validate content size
	if resp.ContentLength > 0 {
		if err := wfs.security.ValidateContentSize(resp.ContentLength, wfs.config.MaxContentSize); err != nil {
			return nil, fmt.Errorf("content size validation failed: %w", err)
		}
	}

	// Read response body with size limit
	limitReader := io.LimitReader(resp.Body, wfs.config.MaxContentSize+1)
	content, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if content was truncated
	truncated := false
	if int64(len(content)) > wfs.config.MaxContentSize {
		content = content[:wfs.config.MaxContentSize]
		truncated = true
	}

	// Extract response headers
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	result := &WebFetchSkillResult{
		URL:           urlStr,
		Content:       string(content),
		ContentType:   resp.Header.Get("Content-Type"),
		StatusCode:    resp.StatusCode,
		ContentLength: int64(len(content)),
		Headers:       headers,
		Cached:        false,
	}

	if truncated {
		result.Truncated = true
	}

	return result, nil
}

// getFromCache retrieves a result from cache if available and not expired
func (wfs *WebFetchSkill) getFromCache(url string) *WebFetchSkillResult {
	if !wfs.config.CacheEnabled {
		return nil
	}

	wfs.cache.mutex.RLock()
	defer wfs.cache.mutex.RUnlock()

	cacheKey := wfs.getCacheKey(url)
	entry, exists := wfs.cache.entries[cacheKey]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Since(entry.timestamp) > wfs.cache.ttl {
		return nil
	}

	// Return a copy of the cached result
	cached := *entry.result
	return &cached
}

// putInCache stores a result in cache
func (wfs *WebFetchSkill) putInCache(url string, result *WebFetchSkillResult) {
	if !wfs.config.CacheEnabled {
		return
	}

	wfs.cache.mutex.Lock()
	defer wfs.cache.mutex.Unlock()

	cacheKey := wfs.getCacheKey(url)
	
	// Create a copy for caching
	cached := *result
	cached.Cached = false // Reset cached flag for storage
	
	wfs.cache.entries[cacheKey] = &cacheEntry{
		result:    &cached,
		timestamp: time.Now(),
	}

	// Clean expired entries periodically
	wfs.cleanExpiredEntries()
}

// getCacheKey generates a cache key for a URL
func (wfs *WebFetchSkill) getCacheKey(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])
}

// cleanExpiredEntries removes expired entries from cache
func (wfs *WebFetchSkill) cleanExpiredEntries() {
	now := time.Now()
	for key, entry := range wfs.cache.entries {
		if now.Sub(entry.timestamp) > wfs.cache.ttl {
			delete(wfs.cache.entries, key)
		}
	}
}

// Validate validates the arguments for the web fetch operation
func (wfs *WebFetchSkill) Validate(arguments map[string]any) error {
	urlRaw, ok := arguments["url"]
	if !ok {
		return fmt.Errorf("url parameter is required")
	}

	urlStr, ok := urlRaw.(string)
	if !ok {
		return fmt.Errorf("url must be a string")
	}

	if strings.TrimSpace(urlStr) == "" {
		return fmt.Errorf("url cannot be empty")
	}

	// Basic URL validation
	if _, err := url.Parse(urlStr); err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Validate optional parameters
	if val, exists := arguments["method"]; exists {
		method, ok := val.(string)
		if !ok {
			return fmt.Errorf("method must be a string")
		}
		
		allowedMethods := []string{"GET", "POST", "PUT", "DELETE", "HEAD"}
		method = strings.ToUpper(method)
		valid := false
		for _, allowed := range allowedMethods {
			if method == allowed {
				valid = true
				break
			}
		}
		
		if !valid {
			return fmt.Errorf("method must be one of: %v", allowedMethods)
		}
	}

	if val, exists := arguments["headers"]; exists {
		if _, ok := val.(map[string]interface{}); !ok {
			return fmt.Errorf("headers must be an object")
		}
	}

	if val, exists := arguments["body"]; exists {
		if _, ok := val.(string); !ok {
			return fmt.Errorf("body must be a string")
		}
	}

	if val, exists := arguments["follow_redirects"]; exists {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("follow_redirects must be a boolean")
		}
	}

	if val, exists := arguments["use_cache"]; exists {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("use_cache must be a boolean")
		}
	}

	return nil
}