package config

import (
	"time"
)

// SkillsConfig holds configuration for all built-in skills
type SkillsConfig struct {
	Enabled       bool               `env:"ENABLED,default=false" description:"Enable built-in skills globally"`
	RequireToS    bool               `env:"REQUIRE_TOS,default=true" description:"Require terms of service acceptance for skill usage"`
	Safety        SafetyConfig       `env:",prefix=SAFETY_" description:"Global safety configuration"`
	ReadSkill     SkillConfig        `env:",prefix=READ_" description:"Read skill configuration"`
	WriteSkill    WriteSkillConfig   `env:",prefix=WRITE_" description:"Write skill configuration"`
	EditSkill     EditSkillConfig    `env:",prefix=EDIT_" description:"Edit skill configuration"`
	MultiEdit     MultiEditConfig    `env:",prefix=MULTI_EDIT_" description:"MultiEdit skill configuration"`
	WebSearch     WebSearchConfig    `env:",prefix=WEB_SEARCH_" description:"WebSearch skill configuration"`
	WebFetch      WebFetchConfig     `env:",prefix=WEB_FETCH_" description:"WebFetch skill configuration"`
}

// SafetyConfig defines global safety settings
type SafetyConfig struct {
	EnableSandbox    bool     `env:"ENABLE_SANDBOX,default=true" description:"Enable path sandboxing for file operations"`
	SandboxPaths     []string `env:"SANDBOX_PATHS" description:"Allowed sandbox directories (comma-separated)"`
	ProtectedPaths   []string `env:"PROTECTED_PATHS" description:"Protected path patterns (comma-separated)"`
	MaxFileSize      int64    `env:"MAX_FILE_SIZE,default=10485760" description:"Maximum file size in bytes (10MB default)"`
	MaxOperationTime time.Duration `env:"MAX_OPERATION_TIME,default=30s" description:"Maximum time for any skill operation"`
}

// SkillConfig defines basic skill configuration
type SkillConfig struct {
	Enabled          bool          `env:"ENABLED,default=false" description:"Enable this skill"`
	RequireApproval  bool          `env:"REQUIRE_APPROVAL,default=false" description:"Require user approval for this skill"`
	RateLimitPerMin  int           `env:"RATE_LIMIT_PER_MIN,default=60" description:"Rate limit per minute"`
	Timeout          time.Duration `env:"TIMEOUT,default=10s" description:"Timeout for skill operations"`
}

// WriteSkillConfig extends SkillConfig for write operations
type WriteSkillConfig struct {
	SkillConfig
	CreateBackups    bool     `env:"CREATE_BACKUPS,default=true" description:"Create backups before writing"`
	AllowedExtensions []string `env:"ALLOWED_EXTENSIONS" description:"Allowed file extensions (comma-separated)"`
}

// EditSkillConfig extends SkillConfig for edit operations
type EditSkillConfig struct {
	SkillConfig
	CreateBackups    bool `env:"CREATE_BACKUPS,default=true" description:"Create backups before editing"`
	MaxEditsPerCall  int  `env:"MAX_EDITS_PER_CALL,default=10" description:"Maximum edits in single call"`
}

// MultiEditConfig extends SkillConfig for multi-edit operations
type MultiEditConfig struct {
	SkillConfig
	CreateBackups    bool `env:"CREATE_BACKUPS,default=true" description:"Create backups before editing"`
	MaxEditsPerCall  int  `env:"MAX_EDITS_PER_CALL,default=50" description:"Maximum edits in single multi-edit call"`
	AtomicOperations bool `env:"ATOMIC_OPERATIONS,default=true" description:"Enable atomic operations (all-or-nothing)"`
}

// WebSearchConfig extends SkillConfig for web search
type WebSearchConfig struct {
	SkillConfig
	MaxResults       int      `env:"MAX_RESULTS,default=10" description:"Maximum search results to return"`
	AllowedEngines   []string `env:"ALLOWED_ENGINES" description:"Allowed search engines (comma-separated)"`
	APIKey           string   `env:"API_KEY" description:"API key for search service"`
}

// WebFetchConfig extends SkillConfig for web fetching
type WebFetchConfig struct {
	SkillConfig
	WhitelistedDomains []string      `env:"WHITELISTED_DOMAINS" description:"Whitelisted domains (comma-separated)"`
	MaxContentSize     int64         `env:"MAX_CONTENT_SIZE,default=1048576" description:"Maximum content size in bytes (1MB default)"`
	FollowRedirects    bool          `env:"FOLLOW_REDIRECTS,default=true" description:"Follow HTTP redirects"`
	UserAgent          string        `env:"USER_AGENT,default=A2A-Agent/1.0" description:"User agent for web requests"`
	CacheEnabled       bool          `env:"CACHE_ENABLED,default=true" description:"Enable response caching"`
	CacheTTL           time.Duration `env:"CACHE_TTL,default=3600s" description:"Cache TTL for web responses"`
}

// GetDefaultSkillsConfig returns a default skills configuration
func GetDefaultSkillsConfig() *SkillsConfig {
	return &SkillsConfig{
		Enabled:    false,
		RequireToS: true,
		Safety: SafetyConfig{
			EnableSandbox:    true,
			SandboxPaths:     []string{},
			ProtectedPaths:   []string{".git", ".env", "*.key", "*.pem", "*.p12", "*.pfx"},
			MaxFileSize:      10 * 1024 * 1024, // 10MB
			MaxOperationTime: 30 * time.Second,
		},
		ReadSkill: SkillConfig{
			Enabled:         false,
			RequireApproval: false,
			RateLimitPerMin: 60,
			Timeout:         10 * time.Second,
		},
		WriteSkill: WriteSkillConfig{
			SkillConfig: SkillConfig{
				Enabled:         false,
				RequireApproval: true,
				RateLimitPerMin: 30,
				Timeout:         15 * time.Second,
			},
			CreateBackups:     true,
			AllowedExtensions: []string{},
		},
		EditSkill: EditSkillConfig{
			SkillConfig: SkillConfig{
				Enabled:         false,
				RequireApproval: true,
				RateLimitPerMin: 30,
				Timeout:         15 * time.Second,
			},
			CreateBackups:   true,
			MaxEditsPerCall: 10,
		},
		MultiEdit: MultiEditConfig{
			SkillConfig: SkillConfig{
				Enabled:         false,
				RequireApproval: true,
				RateLimitPerMin: 20,
				Timeout:         30 * time.Second,
			},
			CreateBackups:    true,
			MaxEditsPerCall:  50,
			AtomicOperations: true,
		},
		WebSearch: WebSearchConfig{
			SkillConfig: SkillConfig{
				Enabled:         false,
				RequireApproval: false,
				RateLimitPerMin: 30,
				Timeout:         10 * time.Second,
			},
			MaxResults:     10,
			AllowedEngines: []string{},
			APIKey:         "",
		},
		WebFetch: WebFetchConfig{
			SkillConfig: SkillConfig{
				Enabled:         false,
				RequireApproval: false,
				RateLimitPerMin: 60,
				Timeout:         15 * time.Second,
			},
			WhitelistedDomains: []string{},
			MaxContentSize:     1024 * 1024, // 1MB
			FollowRedirects:    true,
			UserAgent:          "A2A-Agent/1.0",
			CacheEnabled:       true,
			CacheTTL:           time.Hour,
		},
	}
}