package skills

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// ReadSkill implements file reading capabilities
type ReadSkill struct {
	logger   *zap.Logger
	config   *SkillConfig
	security *SecurityValidator
}

// ReadSkillResult represents the result of a read operation
type ReadSkillResult struct {
	Content   string `json:"content"`
	FilePath  string `json:"file_path"`
	Lines     int    `json:"lines"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
}

// NewReadSkill creates a new Read skill
func NewReadSkill(logger *zap.Logger, config *SkillConfig, security *SecurityValidator) *ReadSkill {
	return &ReadSkill{
		logger:   logger,
		config:   config,
		security: security,
	}
}

// GetName returns the skill name
func (rs *ReadSkill) GetName() string {
	return "read"
}

// GetDescription returns the skill description
func (rs *ReadSkill) GetDescription() string {
	return "Read file contents with optional line range specification. Supports text files and provides metadata about the file."
}

// GetParameters returns the JSON schema for skill parameters
func (rs *ReadSkill) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to read",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "Starting line number (1-based, optional)",
				"minimum":     1,
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Ending line number (1-based, optional)",
				"minimum":     1,
			},
			"max_lines": map[string]any{
				"type":        "integer",
				"description": "Maximum number of lines to read (optional, default 1000)",
				"minimum":     1,
				"default":     1000,
			},
		},
		"required": []string{"file_path"},
	}
}

// IsEnabled returns whether the skill is enabled
func (rs *ReadSkill) IsEnabled() bool {
	return rs.config.Enabled
}

// GetRateLimit returns the rate limit for this skill
func (rs *ReadSkill) GetRateLimit() int {
	return rs.config.RateLimitPerMin
}

// RequiresApproval returns whether this skill requires user approval
func (rs *ReadSkill) RequiresApproval() bool {
	return rs.config.RequireApproval
}

// Execute performs the read operation
func (rs *ReadSkill) Execute(ctx context.Context, arguments map[string]any) (string, error) {
	// Extract and validate arguments
	filePathRaw, ok := arguments["file_path"]
	if !ok {
		return "", fmt.Errorf("file_path parameter is required")
	}

	filePath, ok := filePathRaw.(string)
	if !ok {
		return "", fmt.Errorf("file_path must be a string")
	}

	// Validate file path security
	if err := rs.security.ValidateFilePath(filePath); err != nil {
		return "", fmt.Errorf("security validation failed: %w", err)
	}

	// Validate file size
	if err := rs.security.ValidateFileSize(filePath); err != nil {
		return "", fmt.Errorf("file size validation failed: %w", err)
	}

	// Extract optional line range parameters
	startLine := 1
	endLine := -1
	maxLines := 1000

	if val, exists := arguments["start_line"]; exists {
		if num, ok := val.(float64); ok {
			startLine = int(num)
		}
	}

	if val, exists := arguments["end_line"]; exists {
		if num, ok := val.(float64); ok {
			endLine = int(num)
		}
	}

	if val, exists := arguments["max_lines"]; exists {
		if num, ok := val.(float64); ok {
			maxLines = int(num)
		}
	}

	// Validate line parameters
	if startLine < 1 {
		return "", fmt.Errorf("start_line must be >= 1")
	}
	if endLine != -1 && endLine < startLine {
		return "", fmt.Errorf("end_line must be >= start_line")
	}
	if maxLines < 1 {
		maxLines = 1000
	}

	// Read the file
	result, err := rs.readFile(ctx, filePath, startLine, endLine, maxLines)
	if err != nil {
		rs.logger.Error("failed to read file",
			zap.String("file_path", filePath),
			zap.Error(err))
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Return JSON result
	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	rs.logger.Info("successfully read file",
		zap.String("file_path", filePath),
		zap.Int("lines", result.Lines),
		zap.Int64("size", result.Size))

	return string(jsonData), nil
}

// readFile performs the actual file reading with line range support
func (rs *ReadSkill) readFile(ctx context.Context, filePath string, startLine, endLine, maxLines int) (*ReadSkillResult, error) {
	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// Check for cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Determine if we need to read specific lines or the whole file
	if startLine == 1 && endLine == -1 && maxLines >= 10000 {
		// Read entire file at once if no line restrictions
		return rs.readEntireFile(filePath, fileInfo.Size())
	}

	// Read with line-by-line processing
	return rs.readWithLineRange(file, filePath, startLine, endLine, maxLines, fileInfo.Size())
}

// readEntireFile reads the entire file at once
func (rs *ReadSkill) readEntireFile(filePath string, size int64) (*ReadSkillResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	// Check if content is binary
	if rs.isBinaryContent(content) {
		return nil, fmt.Errorf("file appears to be binary, cannot read as text")
	}

	contentStr := string(content)
	lines := strings.Count(contentStr, "\n")
	if len(contentStr) > 0 && !strings.HasSuffix(contentStr, "\n") {
		lines++ // Count the last line if it doesn't end with newline
	}

	return &ReadSkillResult{
		Content:  contentStr,
		FilePath: filePath,
		Lines:    lines,
		Size:     size,
	}, nil
}

// readWithLineRange reads file with line range support
func (rs *ReadSkill) readWithLineRange(file *os.File, filePath string, startLine, endLine, maxLines int, size int64) (*ReadSkillResult, error) {
	scanner := bufio.NewScanner(file)
	var content strings.Builder
	currentLine := 1
	linesRead := 0
	truncated := false

	for scanner.Scan() {
		// Check if we've reached the start line
		if currentLine >= startLine {
			// Check if we've exceeded the end line
			if endLine != -1 && currentLine > endLine {
				break
			}

			// Check if we've exceeded max lines
			if linesRead >= maxLines {
				truncated = true
				break
			}

			// Add line to content
			if linesRead > 0 {
				content.WriteString("\n")
			}
			content.WriteString(scanner.Text())
			linesRead++
		}
		currentLine++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	result := &ReadSkillResult{
		Content:  content.String(),
		FilePath: filePath,
		Lines:    linesRead,
		Size:     size,
	}

	if truncated {
		result.Truncated = true
	}

	return result, nil
}

// isBinaryContent checks if content appears to be binary
func (rs *ReadSkill) isBinaryContent(content []byte) bool {
	// Check for null bytes in the first 8192 bytes
	checkLength := len(content)
	if checkLength > 8192 {
		checkLength = 8192
	}

	nullBytes := 0
	for i := 0; i < checkLength; i++ {
		if content[i] == 0 {
			nullBytes++
		}
	}

	// If more than 1% of checked bytes are null, consider it binary
	return float64(nullBytes)/float64(checkLength) > 0.01
}

// Validate validates the arguments for the read operation
func (rs *ReadSkill) Validate(arguments map[string]any) error {
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

	// Validate optional parameters
	if val, exists := arguments["start_line"]; exists {
		if num, ok := val.(float64); !ok || num < 1 {
			return fmt.Errorf("start_line must be a positive integer")
		}
	}

	if val, exists := arguments["end_line"]; exists {
		if num, ok := val.(float64); !ok || num < 1 {
			return fmt.Errorf("end_line must be a positive integer")
		}
	}

	if val, exists := arguments["max_lines"]; exists {
		if num, ok := val.(float64); !ok || num < 1 {
			return fmt.Errorf("max_lines must be a positive integer")
		}
	}

	return nil
}
