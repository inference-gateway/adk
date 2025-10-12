package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// WriteSkill implements file writing capabilities
type WriteSkill struct {
	logger   *zap.Logger
	config   *WriteSkillConfig
	security *SecurityValidator
}

// WriteSkillResult represents the result of a write operation
type WriteSkillResult struct {
	FilePath     string `json:"file_path"`
	BytesWritten int    `json:"bytes_written"`
	BackupPath   string `json:"backup_path,omitempty"`
	Created      bool   `json:"created"`
}

// NewWriteSkill creates a new Write skill
func NewWriteSkill(logger *zap.Logger, config *WriteSkillConfig, security *SecurityValidator) *WriteSkill {
	return &WriteSkill{
		logger:   logger,
		config:   config,
		security: security,
	}
}

// GetName returns the skill name
func (ws *WriteSkill) GetName() string {
	return "write"
}

// GetDescription returns the skill description
func (ws *WriteSkill) GetDescription() string {
	return "Write content to a file with optional backup creation. Creates parent directories if they don't exist."
}

// GetParameters returns the JSON schema for skill parameters
func (ws *WriteSkill) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to write",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The content to write to the file",
			},
			"create_backup": map[string]any{
				"type":        "boolean",
				"description": "Whether to create a backup of existing file (default: true)",
				"default":     true,
			},
			"append": map[string]any{
				"type":        "boolean",
				"description": "Whether to append to existing file instead of overwriting (default: false)",
				"default":     false,
			},
		},
		"required": []string{"file_path", "content"},
	}
}

// IsEnabled returns whether the skill is enabled
func (ws *WriteSkill) IsEnabled() bool {
	return ws.config.Enabled
}

// GetRateLimit returns the rate limit for this skill
func (ws *WriteSkill) GetRateLimit() int {
	return ws.config.RateLimitPerMin
}

// RequiresApproval returns whether this skill requires user approval
func (ws *WriteSkill) RequiresApproval() bool {
	return ws.config.RequireApproval
}

// Execute performs the write operation
func (ws *WriteSkill) Execute(ctx context.Context, arguments map[string]any) (string, error) {
	// Extract and validate arguments
	filePathRaw, ok := arguments["file_path"]
	if !ok {
		return "", fmt.Errorf("file_path parameter is required")
	}

	filePath, ok := filePathRaw.(string)
	if !ok {
		return "", fmt.Errorf("file_path must be a string")
	}

	contentRaw, ok := arguments["content"]
	if !ok {
		return "", fmt.Errorf("content parameter is required")
	}

	content, ok := contentRaw.(string)
	if !ok {
		return "", fmt.Errorf("content must be a string")
	}

	// Extract optional parameters
	createBackup := ws.config.CreateBackups // Use config default
	if val, exists := arguments["create_backup"]; exists {
		if backup, ok := val.(bool); ok {
			createBackup = backup
		}
	}

	append := false
	if val, exists := arguments["append"]; exists {
		if appendMode, ok := val.(bool); ok {
			append = appendMode
		}
	}

	// Validate file path security
	if err := ws.security.ValidateFilePath(filePath); err != nil {
		return "", fmt.Errorf("security validation failed: %w", err)
	}

	// Validate file extension if restrictions are configured
	if err := ws.validateFileExtension(filePath); err != nil {
		return "", fmt.Errorf("file extension validation failed: %w", err)
	}

	// Perform the write operation
	result, err := ws.writeFile(ctx, filePath, content, createBackup, append)
	if err != nil {
		ws.logger.Error("failed to write file",
			zap.String("file_path", filePath),
			zap.Error(err))
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Return JSON result
	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	ws.logger.Info("successfully wrote file",
		zap.String("file_path", filePath),
		zap.Int("bytes_written", result.BytesWritten),
		zap.Bool("created", result.Created))

	return string(jsonData), nil
}

// writeFile performs the actual file writing
func (ws *WriteSkill) writeFile(ctx context.Context, filePath, content string, createBackup, append bool) (*WriteSkillResult, error) {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check if file exists
	fileExists := true
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fileExists = false
	}

	var backupPath string
	var err error

	// Create backup if requested and file exists
	if createBackup && fileExists && !append {
		backupPath, err = CreateBackup(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Create parent directories if they don't exist
	parentDir := filepath.Dir(filePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Determine write flags
	flags := os.O_WRONLY | os.O_CREATE
	if append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	// Open file for writing
	file, err := os.OpenFile(filePath, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for writing: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// Write content
	bytesWritten, err := file.WriteString(content)
	if err != nil {
		return nil, fmt.Errorf("failed to write content: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := file.Sync(); err != nil {
		return nil, fmt.Errorf("failed to sync file: %w", err)
	}

	result := &WriteSkillResult{
		FilePath:     filePath,
		BytesWritten: bytesWritten,
		Created:      !fileExists,
	}

	if backupPath != "" {
		result.BackupPath = backupPath
	}

	return result, nil
}

// validateFileExtension validates file extension against allowed extensions
func (ws *WriteSkill) validateFileExtension(filePath string) error {
	if len(ws.config.AllowedExtensions) == 0 {
		return nil // No restrictions
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		// Allow files without extensions if explicitly configured
		for _, allowed := range ws.config.AllowedExtensions {
			if allowed == "" || allowed == "." {
				return nil
			}
		}
		return fmt.Errorf("files without extensions are not allowed")
	}

	// Remove leading dot from extension
	ext = strings.TrimPrefix(ext, ".")

	for _, allowed := range ws.config.AllowedExtensions {
		allowedExt := strings.ToLower(allowed)
		allowedExt = strings.TrimPrefix(allowedExt, ".")
		if ext == allowedExt {
			return nil
		}
	}

	return fmt.Errorf("file extension '%s' is not in allowed list: %v", ext, ws.config.AllowedExtensions)
}

// Validate validates the arguments for the write operation
func (ws *WriteSkill) Validate(arguments map[string]any) error {
	filePathRaw, ok := arguments["file_path"]
	if !ok {
		return fmt.Errorf("file_path parameter is required")
	}

	filePath, ok := filePathRaw.(string)
	if !ok {
		return fmt.Errorf("file_path must be a string")
	}

	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("file_path cannot be empty")
	}

	// Basic path validation
	if !filepath.IsAbs(filePath) {
		return fmt.Errorf("file_path must be an absolute path")
	}

	contentRaw, ok := arguments["content"]
	if !ok {
		return fmt.Errorf("content parameter is required")
	}

	if _, ok := contentRaw.(string); !ok {
		return fmt.Errorf("content must be a string")
	}

	// Validate optional boolean parameters
	if val, exists := arguments["create_backup"]; exists {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("create_backup must be a boolean")
		}
	}

	if val, exists := arguments["append"]; exists {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("append must be a boolean")
		}
	}

	return nil
}
