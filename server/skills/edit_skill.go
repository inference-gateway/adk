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

// EditSkill implements file editing capabilities with string replacement
type EditSkill struct {
	logger   *zap.Logger
	config   *EditSkillConfig
	security *SecurityValidator
}

// EditSkillResult represents the result of an edit operation
type EditSkillResult struct {
	FilePath        string `json:"file_path"`
	BackupPath      string `json:"backup_path,omitempty"`
	ReplacementsMade int    `json:"replacements_made"`
	OriginalContent string `json:"original_content,omitempty"`
	NewContent      string `json:"new_content,omitempty"`
	Preview         bool   `json:"preview"`
}

// NewEditSkill creates a new Edit skill
func NewEditSkill(logger *zap.Logger, config *EditSkillConfig, security *SecurityValidator) *EditSkill {
	return &EditSkill{
		logger:   logger,
		config:   config,
		security: security,
	}
}

// GetName returns the skill name
func (es *EditSkill) GetName() string {
	return "edit"
}

// GetDescription returns the skill description
func (es *EditSkill) GetDescription() string {
	return "Edit file by replacing exact string matches. Creates backup before editing and provides diff preview."
}

// GetParameters returns the JSON schema for skill parameters
func (es *EditSkill) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to edit",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "The exact string to replace (must match exactly including whitespace)",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "The string to replace it with",
			},
			"replace_all": map[string]any{
				"type":        "boolean",
				"description": "Whether to replace all occurrences (default: false, replace only first)",
				"default":     false,
			},
			"create_backup": map[string]any{
				"type":        "boolean",
				"description": "Whether to create a backup before editing (default: true)",
				"default":     true,
			},
			"preview_only": map[string]any{
				"type":        "boolean",
				"description": "Whether to only preview changes without applying them (default: false)",
				"default":     false,
			},
		},
		"required": []string{"file_path", "old_string", "new_string"},
	}
}

// IsEnabled returns whether the skill is enabled
func (es *EditSkill) IsEnabled() bool {
	return es.config.Enabled
}

// GetRateLimit returns the rate limit for this skill
func (es *EditSkill) GetRateLimit() int {
	return es.config.RateLimitPerMin
}

// RequiresApproval returns whether this skill requires user approval
func (es *EditSkill) RequiresApproval() bool {
	return es.config.RequireApproval
}

// Execute performs the edit operation
func (es *EditSkill) Execute(ctx context.Context, arguments map[string]any) (string, error) {
	// Extract and validate arguments
	filePathRaw, ok := arguments["file_path"]
	if !ok {
		return "", fmt.Errorf("file_path parameter is required")
	}

	filePath, ok := filePathRaw.(string)
	if !ok {
		return "", fmt.Errorf("file_path must be a string")
	}

	oldStringRaw, ok := arguments["old_string"]
	if !ok {
		return "", fmt.Errorf("old_string parameter is required")
	}

	oldString, ok := oldStringRaw.(string)
	if !ok {
		return "", fmt.Errorf("old_string must be a string")
	}

	newStringRaw, ok := arguments["new_string"]
	if !ok {
		return "", fmt.Errorf("new_string parameter is required")
	}

	newString, ok := newStringRaw.(string)
	if !ok {
		return "", fmt.Errorf("new_string must be a string")
	}

	// Extract optional parameters
	replaceAll := false
	if val, exists := arguments["replace_all"]; exists {
		if replace, ok := val.(bool); ok {
			replaceAll = replace
		}
	}

	createBackup := es.config.CreateBackups
	if val, exists := arguments["create_backup"]; exists {
		if backup, ok := val.(bool); ok {
			createBackup = backup
		}
	}

	previewOnly := false
	if val, exists := arguments["preview_only"]; exists {
		if preview, ok := val.(bool); ok {
			previewOnly = preview
		}
	}

	// Validate that old_string and new_string are different
	if oldString == newString {
		return "", fmt.Errorf("old_string and new_string must be different")
	}

	// Validate file path security
	if err := es.security.ValidateFilePath(filePath); err != nil {
		return "", fmt.Errorf("security validation failed: %w", err)
	}

	// Validate file size
	if err := es.security.ValidateFileSize(filePath); err != nil {
		return "", fmt.Errorf("file size validation failed: %w", err)
	}

	// Perform the edit operation
	result, err := es.editFile(ctx, filePath, oldString, newString, replaceAll, createBackup, previewOnly)
	if err != nil {
		es.logger.Error("failed to edit file", 
			zap.String("file_path", filePath), 
			zap.Error(err))
		return "", fmt.Errorf("failed to edit file: %w", err)
	}

	// Return JSON result
	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	es.logger.Info("successfully edited file", 
		zap.String("file_path", filePath),
		zap.Int("replacements_made", result.ReplacementsMade),
		zap.Bool("preview_only", previewOnly))

	return string(jsonData), nil
}

// editFile performs the actual file editing
func (es *EditSkill) editFile(ctx context.Context, filePath, oldString, newString string, replaceAll, createBackup, previewOnly bool) (*EditSkillResult, error) {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Read the current file content
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	originalStr := string(originalContent)

	// Perform the replacement
	var newContent string
	var replacementsMade int

	if replaceAll {
		newContent = strings.ReplaceAll(originalStr, oldString, newString)
		// Count occurrences
		replacementsMade = strings.Count(originalStr, oldString)
	} else {
		// Replace only the first occurrence
		if strings.Contains(originalStr, oldString) {
			newContent = strings.Replace(originalStr, oldString, newString, 1)
			replacementsMade = 1
		} else {
			newContent = originalStr
			replacementsMade = 0
		}
	}

	result := &EditSkillResult{
		FilePath:         filePath,
		ReplacementsMade: replacementsMade,
		Preview:          previewOnly,
	}

	// If no replacements were made, return early
	if replacementsMade == 0 {
		return result, nil
	}

	// If preview only, include content for comparison
	if previewOnly {
		result.OriginalContent = originalStr
		result.NewContent = newContent
		return result, nil
	}

	// Create backup if requested
	if createBackup {
		backupPath, err := CreateBackup(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
		result.BackupPath = backupPath
	}

	// Write the modified content
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write modified content: %w", err)
	}

	return result, nil
}

// Validate validates the arguments for the edit operation
func (es *EditSkill) Validate(arguments map[string]any) error {
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

	oldStringRaw, ok := arguments["old_string"]
	if !ok {
		return fmt.Errorf("old_string parameter is required")
	}

	oldString, ok := oldStringRaw.(string)
	if !ok {
		return fmt.Errorf("old_string must be a string")
	}

	if oldString == "" {
		return fmt.Errorf("old_string cannot be empty")
	}

	newStringRaw, ok := arguments["new_string"]
	if !ok {
		return fmt.Errorf("new_string parameter is required")
	}

	newString, ok := newStringRaw.(string)
	if !ok {
		return fmt.Errorf("new_string must be a string")
	}

	if oldString == newString {
		return fmt.Errorf("old_string and new_string must be different")
	}

	// Validate optional boolean parameters
	if val, exists := arguments["replace_all"]; exists {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("replace_all must be a boolean")
		}
	}

	if val, exists := arguments["create_backup"]; exists {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("create_backup must be a boolean")
		}
	}

	if val, exists := arguments["preview_only"]; exists {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("preview_only must be a boolean")
		}
	}

	return nil
}