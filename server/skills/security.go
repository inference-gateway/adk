package skills

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SecurityValidator provides security validation for skills
type SecurityValidator struct {
	config *SafetyConfig
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator(config *SafetyConfig) *SecurityValidator {
	return &SecurityValidator{
		config: config,
	}
}

// ValidateFilePath validates a file path against sandbox restrictions
func (sv *SecurityValidator) ValidateFilePath(filePath string) error {
	if !sv.config.EnableSandbox {
		return nil
	}

	// Clean and resolve the path
	cleanPath, err := filepath.Abs(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	// Check for null bytes (security vulnerability)
	if strings.Contains(filePath, "\x00") {
		return fmt.Errorf("file path contains null bytes")
	}

	// Check for path traversal attempts
	if strings.Contains(filePath, "..") {
		return fmt.Errorf("path traversal detected in file path")
	}

	// Check protected paths
	for _, pattern := range sv.config.ProtectedPaths {
		if matched, _ := filepath.Match(pattern, filepath.Base(cleanPath)); matched {
			return fmt.Errorf("access to protected path denied: %s", pattern)
		}
		// Also check if the path contains the protected pattern
		if strings.Contains(cleanPath, pattern) {
			return fmt.Errorf("access to protected path denied: %s", pattern)
		}
	}

	// If sandbox paths are configured, ensure the path is within one of them
	if len(sv.config.SandboxPaths) > 0 {
		allowed := false
		for _, sandboxPath := range sv.config.SandboxPaths {
			cleanSandbox, err := filepath.Abs(filepath.Clean(sandboxPath))
			if err != nil {
				continue
			}

			// Check if the file path is within the sandbox
			rel, err := filepath.Rel(cleanSandbox, cleanPath)
			if err == nil && !strings.HasPrefix(rel, "..") {
				allowed = true
				break
			}
		}

		if !allowed {
			return fmt.Errorf("file path outside allowed sandbox directories")
		}
	}

	return nil
}

// ValidateFileSize validates file size against limits
func (sv *SecurityValidator) ValidateFileSize(filePath string) error {
	if sv.config.MaxFileSize <= 0 {
		return nil
	}

	info, err := os.Stat(filePath)
	if err != nil {
		// File doesn't exist yet, that's okay for write operations
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() > sv.config.MaxFileSize {
		return fmt.Errorf("file size (%d bytes) exceeds maximum allowed size (%d bytes)",
			info.Size(), sv.config.MaxFileSize)
	}

	return nil
}

// ValidateURL validates a URL for web fetch operations
func (sv *SecurityValidator) ValidateURL(rawURL string, whitelistedDomains []string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTP and HTTPS schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}

	// Check domain whitelist if configured
	if len(whitelistedDomains) > 0 {
		allowed := false
		hostname := strings.ToLower(parsedURL.Hostname())

		for _, domain := range whitelistedDomains {
			domainLower := strings.ToLower(domain)
			// Exact match or subdomain match
			if hostname == domainLower || strings.HasSuffix(hostname, "."+domainLower) {
				allowed = true
				break
			}
		}

		if !allowed {
			return fmt.Errorf("domain %s not in whitelist", hostname)
		}
	}

	return nil
}

// ValidateContentSize validates content size against limits
func (sv *SecurityValidator) ValidateContentSize(size int64, maxSize int64) error {
	if maxSize <= 0 {
		return nil
	}

	if size > maxSize {
		return fmt.Errorf("content size (%d bytes) exceeds maximum allowed size (%d bytes)",
			size, maxSize)
	}

	return nil
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    requestsPerMinute,
		window:   time.Minute,
	}
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Clean old requests
	if times, exists := rl.requests[key]; exists {
		var validTimes []time.Time
		for _, t := range times {
			if t.After(cutoff) {
				validTimes = append(validTimes, t)
			}
		}
		rl.requests[key] = validTimes
	}

	// Check if we're at the limit
	if len(rl.requests[key]) >= rl.limit {
		return false
	}

	// Add current request
	rl.requests[key] = append(rl.requests[key], now)
	return true
}

// TimeoutContext creates a context with timeout based on security configuration
func (sv *SecurityValidator) TimeoutContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, sv.config.MaxOperationTime)
}

// CreateBackupPath generates a backup file path
func CreateBackupPath(originalPath string) string {
	timestamp := time.Now().Format("20060102_150405")
	dir := filepath.Dir(originalPath)
	base := filepath.Base(originalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	backupName := fmt.Sprintf("%s_backup_%s%s", name, timestamp, ext)
	return filepath.Join(dir, backupName)
}

// CreateBackup creates a backup of the original file
func CreateBackup(originalPath string) (string, error) {
	// Check if original file exists
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return "", nil // No backup needed for non-existent file
	}

	backupPath := CreateBackupPath(originalPath)

	// Copy file content
	content, err := os.ReadFile(originalPath)
	if err != nil {
		return "", fmt.Errorf("failed to read original file: %w", err)
	}

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupPath, nil
}
