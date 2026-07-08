// Command generate-providers reads the vendored providers-schema.yaml and
// regenerates every provider-enumerating artifact in this repo between
// marker-delimited regions.
//
// Usage:
//
//	go run ./cmd/generate-providers [--schema providers-schema.yaml]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Schema represents the relevant subset of the openapi.yaml structure.
type Schema struct {
	Components struct {
		Schemas struct {
			Provider struct {
				Enum []string `yaml:"enum"`
			} `yaml:"Provider"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

func main() {
	schemaPath := "providers-schema.yaml"
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "--schema=") {
		schemaPath = strings.TrimPrefix(os.Args[1], "--schema=")
	}

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading schema: %v\n", err)
		os.Exit(1)
	}

	var schema Schema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing schema: %v\n", err)
		os.Exit(1)
	}

	providers := schema.Components.Schemas.Provider.Enum
	if len(providers) == 0 {
		fmt.Fprintln(os.Stderr, "no providers found in schema")
		os.Exit(1)
	}

	// Stable sort for deterministic output
	sort.Strings(providers)

	repoRoot := "."

	// .env.example files that have provider blocks
	envFiles := []string{
		"examples/ai-powered/.env.example",
		"examples/ai-powered-streaming/.env.example",
		"examples/artifacts-autonomous-tool/.env.example",
		"examples/artifacts-with-default-handlers/.env.example",
		"examples/callbacks/.env.example",
		"examples/default-handlers/.env.example",
		"examples/input-required/non-streaming/.env.example",
		"examples/input-required/streaming/.env.example",
		"examples/usage-metadata/.env.example",
	}

	// docker-compose.yaml files that have provider blocks in the gateway service
	composeFiles := []string{
		"examples/ai-powered/docker-compose.yaml",
		"examples/ai-powered-streaming/docker-compose.yaml",
		"examples/callbacks/docker-compose.yaml",
		"examples/default-handlers/docker-compose.yaml",
		"examples/usage-metadata/docker-compose.yaml",
		"examples/artifacts-autonomous-tool/docker-compose.yaml",
		"examples/artifacts-with-default-handlers/docker-compose.yaml",
	}

	// Markers for env/yaml files
	envStartMarker := "# >>> providers: generated from schemas openapi.yaml - do not edit >>>"
	envEndMarker := "# <<< providers <<<"

	// Markers for Markdown files
	mdStartMarker := "<!-- providers:start -->"
	mdEndMarker := "<!-- providers:end -->"

	// Generate env blocks
	envBlock := generateEnvBlock(providers, envStartMarker, envEndMarker)
	for _, rel := range envFiles {
		path := filepath.Join(repoRoot, rel)
		if err := replaceBetween(path, envStartMarker, envEndMarker, envBlock); err != nil {
			fmt.Fprintf(os.Stderr, "error updating %s: %v\n", rel, err)
		} else {
			fmt.Printf("updated %s\n", rel)
		}
	}

	// Generate docker-compose blocks
	composeBlock := generateComposeBlock(providers, envStartMarker, envEndMarker)
	for _, rel := range composeFiles {
		path := filepath.Join(repoRoot, rel)
		if err := replaceBetween(path, envStartMarker, envEndMarker, composeBlock); err != nil {
			fmt.Fprintf(os.Stderr, "error updating %s: %v\n", rel, err)
		} else {
			fmt.Printf("updated %s\n", rel)
		}
	}

	// Update root README.md multi-provider line
	readmePath := filepath.Join(repoRoot, "README.md")
	if err := updateReadmeProviderList(readmePath, providers, mdStartMarker, mdEndMarker); err != nil {
		fmt.Fprintf(os.Stderr, "error updating README.md: %v\n", err)
	} else {
		fmt.Println("updated README.md")
	}

	// Update example READMEs
	exampleReadmes := []string{
		"examples/ai-powered/README.md",
		"examples/ai-powered-streaming/README.md",
	}
	for _, rel := range exampleReadmes {
		path := filepath.Join(repoRoot, rel)
		if err := updateExampleReadme(path, providers, mdStartMarker, mdEndMarker); err != nil {
			fmt.Fprintf(os.Stderr, "error updating %s: %v\n", rel, err)
		} else {
			fmt.Printf("updated %s\n", rel)
		}
	}
}

// generateEnvBlock creates the marker-delimited block for .env.example files.
func generateEnvBlock(providers []string, startMarker, endMarker string) string {
	var b strings.Builder
	b.WriteString(startMarker)
	b.WriteString("\n")
	for _, p := range providers {
		envName := strings.ToUpper(p) + "_API_KEY"
		b.WriteString(envName)
		b.WriteString("=\n")
	}
	b.WriteString(endMarker)
	return b.String()
}

// generateComposeBlock creates the marker-delimited block for docker-compose.yaml files.
func generateComposeBlock(providers []string, startMarker, endMarker string) string {
	var b strings.Builder
	b.WriteString(startMarker)
	b.WriteString("\n")
	for _, p := range providers {
		envName := strings.ToUpper(p) + "_API_KEY"
		fmt.Fprintf(&b, "      %s: ${%s}\n", envName, envName)
	}
	b.WriteString("      ")
	b.WriteString(endMarker)
	return b.String()
}

// replaceBetween finds startMarker and endMarker in the file at path and
// replaces everything between them (inclusive) with newContent.
func replaceBetween(path, startMarker, endMarker, newContent string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	content := string(data)
	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)

	if startIdx == -1 || endIdx == -1 {
		// Markers not found - this file needs them added.
		// Insert after the first line (or at a sensible position).
		return addMarkers(path, startMarker, endMarker, newContent)
	}

	endIdx += len(endMarker)
	if endIdx > len(content) {
		endIdx = len(content)
	}

	newContent = strings.TrimRight(newContent, "\n") + "\n"
	after := ""
	if endIdx < len(content) {
		after = strings.TrimLeft(content[endIdx:], "\n")
		if after != "" {
			after = "\n" + after
		}
	}
	result := content[:startIdx] + newContent + after

	return os.WriteFile(path, []byte(result), 0644)
}

// addMarkers inserts the marker-delimited block into a file that doesn't have them yet.
func addMarkers(path, startMarker, endMarker, blockContent string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	content := string(data)

	// For .env.example files: insert after the "# Inference Gateway" line or at the top
	// For docker-compose.yaml: insert in the gateway service environment section
	// We'll insert after the first line that looks like a comment or header.

	// Find a good insertion point
	insertAfter := findInsertionPoint(content, path)

	block := strings.TrimRight(blockContent, "\n") + "\n"
	result := content[:insertAfter] + "\n" + block + content[insertAfter:]

	return os.WriteFile(path, []byte(result), 0644)
}

// findInsertionPoint finds where to insert the provider block.
func findInsertionPoint(content, path string) int {
	lines := strings.Split(content, "\n")

	if strings.HasSuffix(path, ".env.example") {
		// Insert after the "# Inference Gateway" comment line, or after line 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "# Inference Gateway" || trimmed == "# Inference Gateway Configuration" {
				// Return position after this line (including the newline)
				pos := 0
				for j := 0; j <= i; j++ {
					pos += len(lines[j]) + 1
				}
				return pos
			}
		}
		// Default: after first line
		return strings.Index(content, "\n") + 1
	}

	if strings.HasSuffix(path, "docker-compose.yaml") {
		// Insert in the gateway service's environment section, after the LOG_LEVEL line
		// or after the last _API_KEY line, or at a sensible position.
		// Look for the environment: key under the gateway/inference-gateway service
		inGatewayService := false
		inEnvironment := false
		lastEnvLine := 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "inference-gateway:") || strings.HasPrefix(trimmed, "gateway:") {
				inGatewayService = true
				continue
			}
			if inGatewayService && trimmed == "environment:" {
				inEnvironment = true
				continue
			}
			if inGatewayService && inEnvironment {
				if trimmed == "" || !strings.HasPrefix(trimmed, "#") && !strings.Contains(trimmed, ":") {
					// End of environment section
					pos := 0
					for j := 0; j < i; j++ {
						pos += len(lines[j]) + 1
					}
					return pos
				}
				if strings.Contains(trimmed, "_API_KEY") || strings.Contains(trimmed, "LOG_LEVEL") {
					pos := 0
					for j := 0; j <= i; j++ {
						pos += len(lines[j]) + 1
					}
					lastEnvLine = pos
				}
			}
			if inGatewayService && !inEnvironment && trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "    ") {
				// Left the gateway service
				break
			}
		}
		if lastEnvLine > 0 {
			return lastEnvLine
		}
	}

	// Default: insert after first newline
	idx := strings.Index(content, "\n")
	if idx == -1 {
		return len(content)
	}
	return idx + 1
}

// updateReadmeProviderList updates the "Multi-Provider Support" line in the root README.
func updateReadmeProviderList(path string, providers []string, startMarker, endMarker string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	content := string(data)
	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)

	displayNames := make([]string, len(providers))
	for i, p := range providers {
		displayNames[i] = providerDisplayName(p)
	}

	// Build the provider list text
	providerList := strings.Join(displayNames, ", ")

	if startIdx == -1 || endIdx == -1 {
		// Need to add markers. Find the "Multi-Provider Support" line.
		oldLine := "Works with OpenAI, Ollama, Groq, Cohere, Nvidia, and other LLM providers"
		newLine := fmt.Sprintf("Works with %s, and other LLM providers", providerList)
		newContent := strings.Replace(content, oldLine, newLine, 1)
		if newContent == content {
			return fmt.Errorf("could not find multi-provider line in README.md")
		}
		return os.WriteFile(path, []byte(newContent), 0644)
	}

	// Replace between markers
	replacement := fmt.Sprintf("%s\n%s\n%s", startMarker, providerList, endMarker)
	endIdx += len(endMarker)
	if endIdx > len(content) {
		endIdx = len(content)
	}
	result := content[:startIdx] + replacement
	if endIdx < len(content) {
		result += content[endIdx:]
	}
	return os.WriteFile(path, []byte(result), 0644)
}

// updateExampleReadme updates the "Supported Providers" section in example READMEs.
func updateExampleReadme(path string, providers []string, startMarker, endMarker string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	content := string(data)
	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)

	// Build the provider list as a bullet list
	var b strings.Builder
	b.WriteString(startMarker)
	b.WriteString("\n\n")
	for _, p := range providers {
		name := providerDisplayName(p)
		if p == "nvidia" {
			fmt.Fprintf(&b, "- **%s** (e.g. `nvidia/meta/llama-3.1-8b-instruct`)\n", name)
		} else {
			fmt.Fprintf(&b, "- %s\n", name)
		}
	}
	b.WriteString("\n")
	b.WriteString(endMarker)
	block := b.String()

	if startIdx == -1 || endIdx == -1 {
		// Need to add markers. Find the "Supported Providers" section.
		// Look for the section header and insert after it.
		sectionHeader := "## Supported Providers"
		headerIdx := strings.Index(content, sectionHeader)
		if headerIdx == -1 {
			return fmt.Errorf("could not find 'Supported Providers' section in %s", path)
		}

		// Find the end of the section (next ## or end of file)
		sectionStart := headerIdx + len(sectionHeader)
		nextSection := strings.Index(content[sectionStart:], "\n## ")
		var sectionEnd int
		if nextSection == -1 {
			sectionEnd = len(content)
		} else {
			sectionEnd = sectionStart + nextSection
		}

		// Replace the section content
		newSection := sectionHeader + "\n\n" + block
		result := content[:headerIdx] + newSection
		if sectionEnd < len(content) {
			result += content[sectionEnd:]
		}
		return os.WriteFile(path, []byte(result), 0644)
	}

	// Replace between markers
	endIdx += len(endMarker)
	if endIdx > len(content) {
		endIdx = len(content)
	}
	result := content[:startIdx] + block
	if endIdx < len(content) {
		result += content[endIdx:]
	}
	return os.WriteFile(path, []byte(result), 0644)
}

// providerDisplayName returns a human-readable display name for a provider id.
func providerDisplayName(id string) string {
	switch id {
	case "ollama":
		return "Ollama"
	case "ollama_cloud":
		return "Ollama Cloud"
	case "groq":
		return "Groq"
	case "openai":
		return "OpenAI (GPT models)"
	case "cloudflare":
		return "Cloudflare Workers AI"
	case "cohere":
		return "Cohere"
	case "anthropic":
		return "Anthropic (Claude models)"
	case "deepseek":
		return "DeepSeek"
	case "google":
		return "Google (Gemini)"
	case "mistral":
		return "Mistral"
	case "minimax":
		return "MiniMax"
	case "moonshot":
		return "Moonshot"
	case "nvidia":
		return "NVIDIA NIM"
	default:
		// Fallback: title-case the id
		return strings.ToUpper(id[:1]) + id[1:]
	}
}
