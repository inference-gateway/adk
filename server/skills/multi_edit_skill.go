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

// MultiEditSkill implements multiple file editing capabilities with atomic operations
type MultiEditSkill struct {
	logger   *zap.Logger
	config   *MultiEditConfig
	security *SecurityValidator
}

// EditOperation represents a single edit operation
type EditOperation struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

// MultiEditSkillResult represents the result of a multi-edit operation
type MultiEditSkillResult struct {
	FilePath          string            `json:"file_path"`
	BackupPath        string            `json:"backup_path,omitempty"`
	TotalReplacements int               `json:"total_replacements"`
	OperationResults  []OperationResult `json:"operation_results"`
	Preview           bool              `json:"preview"`
	Success           bool              `json:"success"`
	OriginalContent   string            `json:"original_content,omitempty"`
	NewContent        string            `json:"new_content,omitempty"`
}

// OperationResult represents the result of a single edit operation
type OperationResult struct {
	Operation        EditOperation `json:"operation"`
	ReplacementsMade int           `json:"replacements_made"`
	Success          bool          `json:"success"`
	Error            string        `json:"error,omitempty"`
}

// NewMultiEditSkill creates a new MultiEdit skill
func NewMultiEditSkill(logger *zap.Logger, config *MultiEditConfig, security *SecurityValidator) *MultiEditSkill {
	return &MultiEditSkill{
		logger:   logger,
		config:   config,
		security: security,
	}
}

// GetName returns the skill name
func (mes *MultiEditSkill) GetName() string {
	return "multi_edit"
}

// GetDescription returns the skill description
func (mes *MultiEditSkill) GetDescription() string {
	return "Apply multiple string replacements to a file atomically. All operations succeed or fail together."
}

// GetParameters returns the JSON schema for skill parameters
func (mes *MultiEditSkill) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to edit",
			},
			"edits": map[string]any{
				"type":        "array",
				"description": "Array of edit operations to apply",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"old_string": map[string]any{
							"type":        "string",
							"description": "The exact string to replace",
						},
						"new_string": map[string]any{
							"type":        "string",
							"description": "The string to replace it with",
						},
						"replace_all": map[string]any{
							"type":        "boolean",
							"description": "Whether to replace all occurrences (default: false)",
							"default":     false,
						},
					},
					"required": []string{"old_string", "new_string"},
				},
				"minItems": 1,
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
			"atomic": map[string]any{
				"type":        "boolean",
				"description": "Whether to apply all edits atomically (all succeed or all fail, default: true)",
				"default":     true,
			},
		},
		"required": []string{"file_path", "edits"},
	}
}

// IsEnabled returns whether the skill is enabled
func (mes *MultiEditSkill) IsEnabled() bool {
	return mes.config.Enabled
}

// GetRateLimit returns the rate limit for this skill
func (mes *MultiEditSkill) GetRateLimit() int {
	return mes.config.RateLimitPerMin
}

// RequiresApproval returns whether this skill requires user approval
func (mes *MultiEditSkill) RequiresApproval() bool {
	return mes.config.RequireApproval
}

// Execute performs the multi-edit operation
func (mes *MultiEditSkill) Execute(ctx context.Context, arguments map[string]any) (string, error) {
	// Extract and validate arguments
	filePathRaw, ok := arguments["file_path"]
	if !ok {
		return "", fmt.Errorf("file_path parameter is required")
	}

	filePath, ok := filePathRaw.(string)
	if !ok {
		return "", fmt.Errorf("file_path must be a string")
	}

	editsRaw, ok := arguments["edits"]
	if !ok {
		return "", fmt.Errorf("edits parameter is required")
	}

	editsArray, ok := editsRaw.([]interface{})
	if !ok {
		return "", fmt.Errorf("edits must be an array")
	}

	if len(editsArray) == 0 {
		return "", fmt.Errorf("edits array cannot be empty")
	}

	if len(editsArray) > mes.config.MaxEditsPerCall {
		return "", fmt.Errorf("too many edits: %d (max: %d)", len(editsArray), mes.config.MaxEditsPerCall)
	}

	// Parse edit operations
	editOps, err := mes.parseEditOperations(editsArray)
	if err != nil {
		return "", fmt.Errorf("failed to parse edit operations: %w", err)
	}

	// Extract optional parameters
	createBackup := mes.config.CreateBackups
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

	atomic := mes.config.AtomicOperations
	if val, exists := arguments["atomic"]; exists {
		if atomicMode, ok := val.(bool); ok {
			atomic = atomicMode
		}
	}

	// Validate file path security
	if err := mes.security.ValidateFilePath(filePath); err != nil {
		return "", fmt.Errorf("security validation failed: %w", err)
	}

	// Validate file size
	if err := mes.security.ValidateFileSize(filePath); err != nil {
		return "", fmt.Errorf("file size validation failed: %w", err)
	}

	// Perform the multi-edit operation
	result, err := mes.editFile(ctx, filePath, editOps, createBackup, previewOnly, atomic)
	if err != nil {
		mes.logger.Error("failed to perform multi-edit",
			zap.String("file_path", filePath),
			zap.Error(err))
		return "", fmt.Errorf("failed to perform multi-edit: %w", err)
	}

	// Return JSON result
	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	mes.logger.Info("successfully performed multi-edit",
		zap.String("file_path", filePath),
		zap.Int("total_replacements", result.TotalReplacements),
		zap.Bool("preview_only", previewOnly),
		zap.Bool("success", result.Success))

	return string(jsonData), nil
}

// parseEditOperations parses the edit operations from the raw array
func (mes *MultiEditSkill) parseEditOperations(editsArray []interface{}) ([]EditOperation, error) {
	editOps := make([]EditOperation, len(editsArray))

	for i, editRaw := range editsArray {
		editMap, ok := editRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("edit operation %d must be an object", i)
		}

		// Extract old_string
		oldStringRaw, ok := editMap["old_string"]
		if !ok {
			return nil, fmt.Errorf("edit operation %d missing old_string", i)
		}
		oldString, ok := oldStringRaw.(string)
		if !ok {
			return nil, fmt.Errorf("edit operation %d old_string must be a string", i)
		}

		// Extract new_string
		newStringRaw, ok := editMap["new_string"]
		if !ok {
			return nil, fmt.Errorf("edit operation %d missing new_string", i)
		}
		newString, ok := newStringRaw.(string)
		if !ok {
			return nil, fmt.Errorf("edit operation %d new_string must be a string", i)
		}

		if oldString == newString {
			return nil, fmt.Errorf("edit operation %d: old_string and new_string must be different", i)
		}

		// Extract optional replace_all
		replaceAll := false
		if val, exists := editMap["replace_all"]; exists {
			if replace, ok := val.(bool); ok {
				replaceAll = replace
			}
		}

		editOps[i] = EditOperation{
			OldString:  oldString,
			NewString:  newString,
			ReplaceAll: replaceAll,
		}
	}

	return editOps, nil
}

// editFile performs the actual multi-edit operation
func (mes *MultiEditSkill) editFile(ctx context.Context, filePath string, editOps []EditOperation, createBackup, previewOnly, atomic bool) (*MultiEditSkillResult, error) {
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
	workingContent := originalStr

	result := &MultiEditSkillResult{
		FilePath:         filePath,
		OperationResults: make([]OperationResult, len(editOps)),
		Preview:          previewOnly,
		Success:          true,
	}

	// Apply edits sequentially
	for i, editOp := range editOps {
		opResult := OperationResult{
			Operation: editOp,
			Success:   true,
		}

		// Apply the replacement
		var newContent string
		var replacementsMade int

		if editOp.ReplaceAll {
			newContent = strings.ReplaceAll(workingContent, editOp.OldString, editOp.NewString)
			replacementsMade = strings.Count(workingContent, editOp.OldString)
		} else {
			if strings.Contains(workingContent, editOp.OldString) {
				newContent = strings.Replace(workingContent, editOp.OldString, editOp.NewString, 1)
				replacementsMade = 1
			} else {
				newContent = workingContent
				replacementsMade = 0
			}
		}

		opResult.ReplacementsMade = replacementsMade
		result.OperationResults[i] = opResult
		result.TotalReplacements += replacementsMade

		// If atomic mode and no replacements were made, this could be considered a failure
		if atomic && replacementsMade == 0 {
			result.Success = false
			opResult.Success = false
			opResult.Error = "no matches found for replacement string"
			result.OperationResults[i] = opResult
			break
		}

		// Update working content for next operation
		workingContent = newContent
	}

	// If preview only, include content for comparison
	if previewOnly {
		result.OriginalContent = originalStr
		result.NewContent = workingContent
		return result, nil
	}

	// If atomic mode failed, don't apply any changes
	if atomic && !result.Success {
		return result, nil
	}

	// If no changes were made, return early
	if result.TotalReplacements == 0 {
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
	if err := os.WriteFile(filePath, []byte(workingContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write modified content: %w", err)
	}

	return result, nil
}

// Validate validates the arguments for the multi-edit operation
func (mes *MultiEditSkill) Validate(arguments map[string]any) error {
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

	editsRaw, ok := arguments["edits"]
	if !ok {
		return fmt.Errorf("edits parameter is required")
	}

	editsArray, ok := editsRaw.([]interface{})
	if !ok {
		return fmt.Errorf("edits must be an array")
	}

	if len(editsArray) == 0 {
		return fmt.Errorf("edits array cannot be empty")
	}

	if len(editsArray) > mes.config.MaxEditsPerCall {
		return fmt.Errorf("too many edits: %d (max: %d)", len(editsArray), mes.config.MaxEditsPerCall)
	}

	// Validate each edit operation
	for i, editRaw := range editsArray {
		editMap, ok := editRaw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("edit operation %d must be an object", i)
		}

		if _, ok := editMap["old_string"]; !ok {
			return fmt.Errorf("edit operation %d missing old_string", i)
		}

		if _, ok := editMap["new_string"]; !ok {
			return fmt.Errorf("edit operation %d missing new_string", i)
		}

		oldString, ok := editMap["old_string"].(string)
		if !ok {
			return fmt.Errorf("edit operation %d old_string must be a string", i)
		}

		newString, ok := editMap["new_string"].(string)
		if !ok {
			return fmt.Errorf("edit operation %d new_string must be a string", i)
		}

		if oldString == "" {
			return fmt.Errorf("edit operation %d old_string cannot be empty", i)
		}

		if oldString == newString {
			return fmt.Errorf("edit operation %d: old_string and new_string must be different", i)
		}

		if val, exists := editMap["replace_all"]; exists {
			if _, ok := val.(bool); !ok {
				return fmt.Errorf("edit operation %d replace_all must be a boolean", i)
			}
		}
	}

	// Validate optional boolean parameters
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

	if val, exists := arguments["atomic"]; exists {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("atomic must be a boolean")
		}
	}

	return nil
}
