package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/adk/examples/artifacts-with-default-handlers/server/config"
)

// Artifacts with Default Handlers and Filesystem Server Example
//
// This example demonstrates an A2A server using default task handlers with
// automatic artifact extraction and a filesystem-based artifacts server.
// Tools create artifacts using ArtifactHelper, the default handlers automatically
// extract and attach them to tasks, and the artifacts server enables upload/download.
//
// Features:
// - Default task handlers with automatic artifact extraction
// - Filesystem-based artifacts storage server
// - Tools that create downloadable artifacts
// - Client can upload files that are processed by the agent
// - Client can download generated artifacts via HTTP endpoints
//
// To run: go run main.go
func main() {
	fmt.Println("🔧 Starting Artifacts with Default Handlers A2A Server...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	// Create configuration with defaults
	cfg := &config.Config{
		Environment: "development",
		A2A: serverConfig.Config{
			AgentName:        server.BuildAgentName,
			AgentDescription: server.BuildAgentDescription,
			AgentVersion:     server.BuildAgentVersion,
			Debug:            false,
			CapabilitiesConfig: serverConfig.CapabilitiesConfig{
				Streaming:              true,
				PushNotifications:      false,
				StateTransitionHistory: false,
			},
			QueueConfig: serverConfig.QueueConfig{
				CleanupInterval: 5 * time.Minute,
			},
			ServerConfig: serverConfig.ServerConfig{
				Port: "8080",
			},
		},
	}

	// Load configuration from environment variables
	ctx := context.Background()
	if err := envconfig.Process(ctx, cfg); err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	// Enable artifacts server with filesystem storage
	cfg.A2A.ArtifactsConfig = serverConfig.ArtifactsConfig{
		Enable: true,
		// ServerConfig and StorageConfig will be populated from environment variables via envconfig
		StorageConfig: serverConfig.ArtifactsStorageConfig{
			Provider: "filesystem",
			BasePath: "./artifacts",
		},
	}

	// Log configuration info
	logger.Info("configuration loaded",
		zap.String("environment", cfg.Environment),
		zap.String("agent_name", cfg.A2A.AgentName),
		zap.String("a2a_port", cfg.A2A.ServerConfig.Port),
		zap.String("artifacts_port", cfg.A2A.ArtifactsConfig.ServerConfig.Port),
		zap.Bool("artifacts_enabled", cfg.A2A.ArtifactsConfig.Enable),
		zap.String("storage_provider", cfg.A2A.ArtifactsConfig.StorageConfig.Provider),
		zap.String("storage_path", cfg.A2A.ArtifactsConfig.StorageConfig.BasePath),
		zap.Bool("debug", cfg.A2A.Debug),
		zap.String("provider", cfg.A2A.AgentConfig.Provider),
		zap.String("model", cfg.A2A.AgentConfig.Model),
	)

	// Create artifacts server for file storage and retrieval
	artifactsServer, err := server.
		NewArtifactsServerBuilder(&cfg.A2A.ArtifactsConfig, logger).
		Build()
	if err != nil {
		logger.Fatal("failed to create artifacts server", zap.Error(err))
	}

	// Create toolbox with only artifact-creating tools
	toolBox := server.NewToolBox()

	// Add file processor tool that can process uploaded files
	fileProcessorTool := server.NewBasicTool(
		"process_uploaded_file",
		"Process an uploaded file and create an analysis report",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filename": map[string]any{
					"type":        "string",
					"description": "The name of the uploaded file",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content of the uploaded file",
				},
			},
			"required": []string{"filename", "content"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			// Extract Task and ArtifactHelper from context
			task, ok := ctx.Value(server.TaskContextKey).(*types.Task)
			if !ok {
				return "Failed to get task from context", fmt.Errorf("task not found in context")
			}

			artifactHelper, ok := ctx.Value(server.ArtifactHelperContextKey).(*server.ArtifactHelper)
			if !ok {
				return "Failed to get artifact helper from context", fmt.Errorf("artifact helper not found in context")
			}

			filename := args["filename"].(string)
			content := args["content"].(string)

			// Generate analysis report based on uploaded file
			analysisReport := fmt.Sprintf(`# File Analysis Report

## Uploaded File
- **Filename**: %s
- **Size**: %d bytes
- **Processed at**: %s

## Content Analysis

### Raw Content
`+"```"+`
%s
`+"```"+`

### Summary
This file has been successfully uploaded and processed by the A2A server.
The content has been analyzed and this report has been generated as a downloadable artifact.

### Key Insights
- File was uploaded via the A2A protocol
- Content has been preserved and analyzed
- This demonstrates bidirectional artifact exchange

## Conclusions
The file upload and processing demonstrates the complete artifact lifecycle:
1. Client uploads a file as part of the message
2. Server processes the file content
3. Server generates an analysis report
4. Report is stored as a downloadable artifact
5. Client can download the generated artifact

---
*Generated by A2A Artifacts Server with Default Handlers*
`, filename, len(content), time.Now().Format(time.RFC3339), content)

			mimeType := "text/markdown"
			reportFilename := fmt.Sprintf("analysis_%s.md", filename)

			// First create a placeholder artifact to get the artifact ID
			artifact := artifactHelper.CreateFileArtifactFromBytes(
				fmt.Sprintf("Analysis Report for %s", filename),
				fmt.Sprintf("Detailed analysis of the uploaded file: %s", filename),
				reportFilename,
				[]byte(analysisReport),
				&mimeType,
			)

			// Store the analysis report using the artifact ID as directory name
			var artifactURL string
			if artifactsServer != nil {
				storage := artifactsServer.GetStorage()

				url, err := storage.Store(ctx, artifact.ArtifactID, reportFilename, strings.NewReader(analysisReport))
				if err == nil {
					artifactURL = url
					logger.Info("artifact stored in filesystem",
						zap.String("artifact_id", artifact.ArtifactID),
						zap.String("filename", reportFilename),
						zap.String("url", url))
				}
			}

			// Create URI-based artifact if we have a URL, otherwise keep bytes-based
			if artifactURL != "" {
				artifact = artifactHelper.CreateFileArtifactFromURI(
					fmt.Sprintf("Analysis Report for %s", filename),
					fmt.Sprintf("Detailed analysis of the uploaded file: %s", filename),
					reportFilename,
					artifactURL,
					&mimeType,
				)
			}

			// Directly add artifact to task
			artifactHelper.AddArtifactToTask(task, artifact)

			return fmt.Sprintf("File '%s' processed successfully, analysis report created.", filename), nil
		},
	)
	toolBox.AddTool(fileProcessorTool)

	// Add report generator tool that creates markdown artifacts
	reportTool := server.NewBasicTool(
		"generate_report",
		"Generate a comprehensive report and save it as a downloadable artifact",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"topic": map[string]any{
					"type":        "string",
					"description": "The topic for the report",
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Report format (markdown, json, xml)",
					"default":     "markdown",
				},
			},
			"required": []string{"topic"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			// Extract Task and ArtifactHelper from context
			task, ok := ctx.Value(server.TaskContextKey).(*types.Task)
			if !ok {
				return "Failed to get task from context", fmt.Errorf("task not found in context")
			}

			artifactHelper, ok := ctx.Value(server.ArtifactHelperContextKey).(*server.ArtifactHelper)
			if !ok {
				return "Failed to get artifact helper from context", fmt.Errorf("artifact helper not found in context")
			}

			topic := args["topic"].(string)
			format := "markdown"
			if f, ok := args["format"].(string); ok {
				format = f
			}

			// Create report content based on format
			var content string
			var mimeType string
			var filename string

			switch format {
			case "json":
				mimeType = "application/json"
				filename = "report.json"
				content = fmt.Sprintf(`{
  "title": "Analysis Report: %s",
  "generated_at": "%s",
  "topic": "%s",
  "sections": [
    {
      "heading": "Executive Summary",
      "content": "This is a comprehensive analysis of %s generated automatically."
    },
    {
      "heading": "Key Findings",
      "content": "Our analysis reveals several important insights about %s."
    },
    {
      "heading": "Recommendations",
      "content": "Based on the analysis, we recommend further investigation into %s."
    }
  ],
  "metadata": {
    "format": "json",
    "version": "1.0"
  }
}`, topic, time.Now().Format(time.RFC3339), topic, topic, topic, topic)

			case "xml":
				mimeType = "application/xml"
				filename = "report.xml"
				content = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<report>
  <title>Analysis Report: %s</title>
  <generated_at>%s</generated_at>
  <topic>%s</topic>
  <sections>
    <section>
      <heading>Executive Summary</heading>
      <content>This is a comprehensive analysis of %s generated automatically.</content>
    </section>
    <section>
      <heading>Key Findings</heading>
      <content>Our analysis reveals several important insights about %s.</content>
    </section>
    <section>
      <heading>Recommendations</heading>
      <content>Based on the analysis, we recommend further investigation into %s.</content>
    </section>
  </sections>
  <metadata>
    <format>xml</format>
    <version>1.0</version>
  </metadata>
</report>`, topic, time.Now().Format(time.RFC3339), topic, topic, topic, topic)

			default: // markdown
				mimeType = "text/markdown"
				filename = "report.md"
				content = fmt.Sprintf(`# Analysis Report: %s

## Executive Summary

This is a comprehensive analysis of %s generated automatically by the artifact-creating tool.

## Key Findings

Our analysis reveals several important insights about %s:

- **Finding 1**: The topic shows significant potential for further development
- **Finding 2**: Current implementations could benefit from optimization
- **Finding 3**: There are opportunities for innovative approaches

## Detailed Analysis

### Background
%s represents an important area of study that requires careful consideration of multiple factors.

### Methodology
Our analysis employed a systematic approach to evaluate all relevant aspects of %s.

### Results
The results indicate that %s has both challenges and opportunities that warrant attention.

## Recommendations

Based on our analysis, we recommend:

1. **Further Research**: Conduct deeper investigation into %s
2. **Implementation**: Develop pilot programs to test new approaches
3. **Monitoring**: Establish metrics to track progress and outcomes

## Conclusion

This report demonstrates how tools can create artifacts that are automatically extracted and attached to tasks by the default handlers.

---
*Generated at: %s*
*Report ID: %s*
`, topic, topic, topic, topic, topic, topic, topic, time.Now().Format(time.RFC3339), "auto-generated")
			}

			// First create a placeholder artifact to get the artifact ID
			artifact := artifactHelper.CreateFileArtifactFromBytes(
				fmt.Sprintf("%s Analysis Report", strings.Title(topic)),
				fmt.Sprintf("Comprehensive analysis report about %s in %s format", topic, format),
				filename,
				[]byte(content),
				&mimeType,
			)

			// Store the report using the artifact ID as directory name
			var artifactURL string
			if artifactsServer != nil {
				storage := artifactsServer.GetStorage()

				url, err := storage.Store(ctx, artifact.ArtifactID, filename, strings.NewReader(content))
				if err == nil {
					artifactURL = url
					logger.Info("artifact stored in filesystem",
						zap.String("artifact_id", artifact.ArtifactID),
						zap.String("filename", filename),
						zap.String("url", url))
				}
			}

			// Create URI-based artifact if we have a URL, otherwise keep bytes-based
			if artifactURL != "" {
				artifact = artifactHelper.CreateFileArtifactFromURI(
					*artifact.Name,
					*artifact.Description,
					filename,
					artifactURL,
					&mimeType,
				)
			}

			// Directly add artifact to task
			artifactHelper.AddArtifactToTask(task, artifact)

			return fmt.Sprintf("Report '%s' generated successfully in %s format.", topic, format), nil
		},
	)
	toolBox.AddTool(reportTool)

	// Add diagram creator tool that creates PlantUML diagrams
	diagramTool := server.NewBasicTool(
		"create_diagram",
		"Create a diagram and save it as a downloadable artifact",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"diagram_type": map[string]any{
					"type":        "string",
					"description": "Type of diagram (sequence, class, activity, component)",
					"default":     "sequence",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Diagram title",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Diagram description",
				},
			},
			"required": []string{"title"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			// Extract Task and ArtifactHelper from context
			task, ok := ctx.Value(server.TaskContextKey).(*types.Task)
			if !ok {
				return "Failed to get task from context", fmt.Errorf("task not found in context")
			}

			artifactHelper, ok := ctx.Value(server.ArtifactHelperContextKey).(*server.ArtifactHelper)
			if !ok {
				return "Failed to get artifact helper from context", fmt.Errorf("artifact helper not found in context")
			}

			title := args["title"].(string)
			diagramType := "sequence"
			if dt, ok := args["diagram_type"].(string); ok {
				diagramType = dt
			}
			description := fmt.Sprintf("Generated %s diagram", diagramType)
			if desc, ok := args["description"].(string); ok {
				description = desc
			}

			// Generate PlantUML content based on diagram type
			var plantumlContent string
			filename := fmt.Sprintf("%s_diagram.puml", diagramType)

			switch diagramType {
			case "class":
				plantumlContent = fmt.Sprintf(`@startuml
title %s

class User {
  +String name
  +String email
  +authenticate()
  +logout()
}

class Session {
  +String sessionId
  +DateTime createdAt
  +DateTime expiresAt
  +validate()
}

class Database {
  +connect()
  +query()
  +disconnect()
}

User ||--o{ Session : creates
Session --> Database : stores

note top of User : %s
@enduml`, title, description)

			case "activity":
				plantumlContent = fmt.Sprintf(`@startuml
title %s

start
:User requests action;
:Validate input;
if (Valid input?) then (yes)
  :Process request;
  :Generate result;
  :Return success;
else (no)
  :Return error;
endif
stop

note right : %s
@enduml`, title, description)

			case "component":
				plantumlContent = fmt.Sprintf(`@startuml
title %s

package "Frontend" {
  [Web UI]
  [Mobile App]
}

package "Backend" {
  [API Gateway]
  [Business Logic]
  [Database Layer]
}

package "External Services" {
  [Payment Service]
  [Email Service]
}

[Web UI] --> [API Gateway]
[Mobile App] --> [API Gateway]
[API Gateway] --> [Business Logic]
[Business Logic] --> [Database Layer]
[Business Logic] --> [Payment Service]
[Business Logic] --> [Email Service]

note top of "Backend" : %s
@enduml`, title, description)

			default: // sequence
				plantumlContent = fmt.Sprintf(`@startuml
title %s

actor User
participant Frontend
participant Backend
participant Database

User -> Frontend: Request
Frontend -> Backend: API Call
Backend -> Database: Query
Database -> Backend: Response
Backend -> Frontend: Data
Frontend -> User: Display

note over User,Database : %s
@enduml`, title, description)
			}

			mimeType := "text/plain"

			// First create a placeholder artifact to get the artifact ID
			artifact := artifactHelper.CreateFileArtifactFromBytes(
				fmt.Sprintf("%s - %s Diagram", title, strings.Title(diagramType)),
				fmt.Sprintf("PlantUML %s diagram: %s", diagramType, description),
				filename,
				[]byte(plantumlContent),
				&mimeType,
			)

			// Store the diagram using the artifact ID as directory name
			var artifactURL string
			if artifactsServer != nil {
				storage := artifactsServer.GetStorage()

				url, err := storage.Store(ctx, artifact.ArtifactID, filename, strings.NewReader(plantumlContent))
				if err == nil {
					artifactURL = url
					logger.Info("artifact stored in filesystem",
						zap.String("artifact_id", artifact.ArtifactID),
						zap.String("filename", filename),
						zap.String("url", url))
				}
			}

			// Create URI-based artifact if we have a URL, otherwise keep bytes-based
			if artifactURL != "" {
				artifact = artifactHelper.CreateFileArtifactFromURI(
					*artifact.Name,
					*artifact.Description,
					filename,
					artifactURL,
					&mimeType,
				)
			}

			// Directly add artifact to task
			artifactHelper.AddArtifactToTask(task, artifact)

			return fmt.Sprintf("Diagram '%s' created successfully as a %s diagram.", title, diagramType), nil
		},
	)
	toolBox.AddTool(diagramTool)

	// Add data export tool that creates CSV artifacts
	dataExportTool := server.NewBasicTool(
		"export_data",
		"Export data in various formats as downloadable artifacts",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dataset": map[string]any{
					"type":        "string",
					"description": "Name of the dataset to export",
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Export format (csv, json, xml)",
					"default":     "csv",
				},
			},
			"required": []string{"dataset"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			// Extract Task and ArtifactHelper from context
			task, ok := ctx.Value(server.TaskContextKey).(*types.Task)
			if !ok {
				return "Failed to get task from context", fmt.Errorf("task not found in context")
			}

			artifactHelper, ok := ctx.Value(server.ArtifactHelperContextKey).(*server.ArtifactHelper)
			if !ok {
				return "Failed to get artifact helper from context", fmt.Errorf("artifact helper not found in context")
			}

			dataset := args["dataset"].(string)
			format := "csv"
			if f, ok := args["format"].(string); ok {
				format = f
			}

			// Generate sample data export based on format
			var content string
			var mimeType string
			var filename string

			switch format {
			case "json":
				mimeType = "application/json"
				filename = fmt.Sprintf("%s_export.json", dataset)
				content = fmt.Sprintf(`{
  "dataset": "%s",
  "exported_at": "%s",
  "total_records": 3,
  "data": [
    {"id": 1, "name": "Sample Item 1", "value": 100, "category": "%s"},
    {"id": 2, "name": "Sample Item 2", "value": 200, "category": "%s"},
    {"id": 3, "name": "Sample Item 3", "value": 300, "category": "%s"}
  ],
  "metadata": {
    "format": "json",
    "version": "1.0"
  }
}`, dataset, time.Now().Format(time.RFC3339), dataset, dataset, dataset)

			case "xml":
				mimeType = "application/xml"
				filename = fmt.Sprintf("%s_export.xml", dataset)
				content = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<export>
  <dataset>%s</dataset>
  <exported_at>%s</exported_at>
  <total_records>3</total_records>
  <data>
    <record>
      <id>1</id>
      <name>Sample Item 1</name>
      <value>100</value>
      <category>%s</category>
    </record>
    <record>
      <id>2</id>
      <name>Sample Item 2</name>
      <value>200</value>
      <category>%s</category>
    </record>
    <record>
      <id>3</id>
      <name>Sample Item 3</name>
      <value>300</value>
      <category>%s</category>
    </record>
  </data>
</export>`, dataset, time.Now().Format(time.RFC3339), dataset, dataset, dataset)

			default: // csv
				mimeType = "text/csv"
				filename = fmt.Sprintf("%s_export.csv", dataset)
				content = fmt.Sprintf(`id,name,value,category,created_at
1,"Sample Item 1",100,"%s","%s"
2,"Sample Item 2",200,"%s","%s"
3,"Sample Item 3",300,"%s","%s"`, dataset, time.Now().Format("2006-01-02"), dataset, time.Now().Format("2006-01-02"), dataset, time.Now().Format("2006-01-02"))
			}

			// First create a placeholder artifact to get the artifact ID
			artifact := artifactHelper.CreateFileArtifactFromBytes(
				fmt.Sprintf("%s Dataset Export", strings.Title(dataset)),
				fmt.Sprintf("Data export of %s dataset in %s format", dataset, format),
				filename,
				[]byte(content),
				&mimeType,
			)

			// Store the data export using the artifact ID as directory name
			var artifactURL string
			if artifactsServer != nil {
				storage := artifactsServer.GetStorage()

				url, err := storage.Store(ctx, artifact.ArtifactID, filename, strings.NewReader(content))
				if err == nil {
					artifactURL = url
					logger.Info("artifact stored in filesystem",
						zap.String("artifact_id", artifact.ArtifactID),
						zap.String("filename", filename),
						zap.String("url", url))
				}
			}

			// Create URI-based artifact if we have a URL, otherwise keep bytes-based
			if artifactURL != "" {
				artifact = artifactHelper.CreateFileArtifactFromURI(
					fmt.Sprintf("%s Dataset Export", strings.Title(dataset)),
					fmt.Sprintf("Data export of %s dataset in %s format", dataset, format),
					filename,
					artifactURL,
					&mimeType,
				)
			}

			// Directly add artifact to task
			artifactHelper.AddArtifactToTask(task, artifact)

			return fmt.Sprintf("Data export '%s' completed successfully in %s format.", dataset, format), nil
		},
	)
	toolBox.AddTool(dataExportTool)

	// Create AI agent with LLM client
	llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.A2A.AgentConfig, logger)
	if err != nil {
		logger.Fatal("failed to create LLM client", zap.Error(err))
	}

	// Create AI agent with the configured LLM
	agent, err := server.NewAgentBuilder(logger).
		WithConfig(&cfg.A2A.AgentConfig).
		WithLLMClient(llmClient).
		WithSystemPrompt("You are a helpful AI assistant with access to tools that create downloadable artifacts and process uploaded files. When users upload files, use the process_uploaded_file tool to analyze them. When users request reports, diagrams, or data exports, use the appropriate tools to create these as artifacts. The artifacts are stored on a filesystem server and can be downloaded by clients. The default task handlers will automatically extract and attach the artifacts to your responses. Be concise and helpful in your responses.").
		WithMaxChatCompletion(10).
		WithToolBox(toolBox).
		Build()
	if err != nil {
		logger.Fatal("failed to create AI agent", zap.Error(err))
	}

	// Build server with AI agent and default handlers that automatically extract artifacts
	// Note: This example requires an AI agent - it does not support non-AI mode
	serverBuilder := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithAgent(agent).
		WithDefaultTaskHandlers() // This enables automatic artifact extraction with AI

	// Build and start server
	a2aServer, err := serverBuilder.
		WithAgentCard(types.AgentCard{
			Name:            cfg.A2A.AgentName,
			Description:     cfg.A2A.AgentDescription,
			Version:         cfg.A2A.AgentVersion,
			URL:             fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port),
			ProtocolVersion: "0.3.0",
			Capabilities: types.AgentCapabilities{
				Streaming:              &cfg.A2A.CapabilitiesConfig.Streaming,
				PushNotifications:      &cfg.A2A.CapabilitiesConfig.PushNotifications,
				StateTransitionHistory: &cfg.A2A.CapabilitiesConfig.StateTransitionHistory,
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills: []types.AgentSkill{
				{
					Name:        "Report Generation",
					Description: "Generate comprehensive reports in various formats as downloadable artifacts",
				},
				{
					Name:        "Diagram Creation",
					Description: "Create PlantUML diagrams for documentation and visualization",
				},
				{
					Name:        "Data Export",
					Description: "Export data in CSV, JSON, and XML formats",
				},
			},
		}).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	logger.Info("✅ servers created with default handlers and artifact extraction")

	// Start servers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start artifacts server
	go func() {
		if err := artifactsServer.Start(ctx); err != nil {
			logger.Warn("artifacts server failed to start", zap.Error(err))
		}
	}()

	// Start A2A server
	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("A2A server failed to start", zap.Error(err))
		}
	}()

	logger.Info("🌐 A2A server running on port " + cfg.A2A.ServerConfig.Port)
	logger.Info("📁 Artifacts server running on port " + cfg.A2A.ArtifactsConfig.ServerConfig.Port)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Stop A2A server
	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("A2A server shutdown error", zap.Error(err))
	}

	// Stop artifacts server
	if err := artifactsServer.Stop(shutdownCtx); err != nil {
		logger.Error("artifacts server shutdown error", zap.Error(err))
	}

	logger.Info("✅ goodbye!")
}
