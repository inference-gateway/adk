package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// RunWithStream processes a conversation and returns a streaming response with iterative tool calling support
func (a *OpenAICompatibleAgentImpl) RunWithStream(ctx context.Context, messages []types.Message) (<-chan cloudevents.Event, error) {
	if a.llmClient == nil {
		return nil, fmt.Errorf("no LLM client configured for agent")
	}

	var tools []sdk.ChatCompletionTool
	if a.toolBox != nil {
		tools = a.toolBox.GetTools()
	}

	var taskID *string
	var contextID *string
	if task, ok := ctx.Value(TaskContextKey).(*types.Task); ok && task != nil {
		taskID = &task.ID
		contextID = &task.ContextID
	}

	var usageTracker *UsageTracker
	if tracker, ok := ctx.Value(UsageTrackerContextKey).(*UsageTracker); ok && tracker != nil {
		usageTracker = tracker
	} else {
		usageTracker = NewUsageTracker()
	}

	outputChan := make(chan cloudevents.Event, 100)

	go func() {
		defer close(outputChan)

		callbackCtx := a.createCallbackContext(taskID, contextID)
		executor := a.GetCallbackExecutor()
		if override := executor.ExecuteBeforeAgent(ctx, callbackCtx); override != nil {
			a.logger.Debug("BeforeAgent callback returned override, skipping agent execution")
			completedStatusEvent := cloudevents.NewEvent()
			completedStatusEvent.SetType(types.EventTaskStatusChanged)
			if err := completedStatusEvent.SetData(cloudevents.ApplicationJSON, types.TaskStatus{
				State:   types.TaskStateCompleted,
				Message: override,
			}); err != nil {
				a.logger.Error("failed to set completed status event data", zap.Error(err))
				return
			}
			outputChan <- completedStatusEvent
			return
		}

		statusEvent := cloudevents.NewEvent()
		statusEvent.SetType(types.EventTaskStatusChanged)
		if err := statusEvent.SetData(cloudevents.ApplicationJSON, types.TaskStatus{
			State: types.TaskStateWorking,
		}); err != nil {
			a.logger.Error("failed to set status event data", zap.Error(err))
			return
		}
		outputChan <- statusEvent

		currentMessages := make([]types.Message, len(messages))
		copy(currentMessages, messages)

		var finalAssistantMessage *types.Message

		for iteration := 1; iteration <= a.config.MaxChatCompletionIterations; iteration++ {
			usageTracker.IncrementIteration()

			a.logger.Debug("starting streaming iteration",
				zap.Int("iteration", iteration),
				zap.Int("message_count", len(currentMessages)))

			sdkMessages, err := a.converter.ConvertToSDK(currentMessages)
			if err != nil {
				a.logger.Error("failed to convert messages to SDK format", zap.Error(err))
				return
			}

			if a.config != nil && a.config.SystemPrompt != "" {
				systemMessage, err := sdk.NewTextMessage(sdk.System, a.config.SystemPrompt)
				if err != nil {
					a.logger.Error("failed to create system message", zap.Error(err))
					return
				}
				sdkMessages = append([]sdk.Message{systemMessage}, sdkMessages...)
			}

			llmRequest := &LLMRequest{
				Contents: currentMessages,
				Config: &LLMConfig{
					SystemInstruction: nil,
				},
			}
			if a.config != nil && a.config.SystemPrompt != "" {
				sysMsg := &types.Message{
					Role: "system",
					Parts: []types.Part{
						map[string]any{"kind": "text", "text": a.config.SystemPrompt},
					},
				}
				llmRequest.Config.SystemInstruction = sysMsg
			}

			var beforeModelOverride *LLMResponse
			if override := executor.ExecuteBeforeModel(ctx, callbackCtx, llmRequest); override != nil {
				a.logger.Debug("BeforeModel callback returned override, skipping LLM call")
				beforeModelOverride = override
			}

			var streamResponseChan <-chan *sdk.CreateChatCompletionStreamResponse
			var streamErrorChan <-chan error

			if beforeModelOverride == nil {
				streamResponseChan, streamErrorChan = a.llmClient.CreateStreamingChatCompletion(ctx, sdkMessages, tools...)
			}

			var fullContent string
			toolCallAccumulator := make(map[string]*sdk.ChatCompletionMessageToolCall)
			var assistantMessage *types.Message
			var toolResultMessages []types.Message
			toolResults := make(map[string]*types.Message)
			skipStreaming := false

			if beforeModelOverride != nil && beforeModelOverride.Content != nil {
				assistantMessage = beforeModelOverride.Content
				assistantMessage.TaskID = taskID
				assistantMessage.ContextID = contextID

				llmResponse := &LLMResponse{Content: assistantMessage}
				if modified := executor.ExecuteAfterModel(ctx, callbackCtx, llmResponse); modified != nil && modified.Content != nil {
					assistantMessage = modified.Content
					assistantMessage.TaskID = taskID
					assistantMessage.ContextID = contextID
				}

				for _, part := range assistantMessage.Parts {
					if partMap, ok := part.(map[string]any); ok {
						if text, exists := partMap["text"].(string); exists {
							fullContent = text
							break
						}
					}
				}

				currentMessages = append(currentMessages, *assistantMessage)
				iterationEvent := types.NewIterationCompletedEvent(iteration, "streaming-task", assistantMessage)
				select {
				case outputChan <- iterationEvent:
				case <-ctx.Done():
					return
				}

				skipStreaming = true
			}

			streaming := !skipStreaming
			for streaming {
				select {
				case <-ctx.Done():
					a.logger.Info("streaming context cancelled, preserving partial state",
						zap.Int("iteration", iteration),
						zap.Bool("has_assistant_message", assistantMessage != nil),
						zap.Int("content_length", len(fullContent)),
						zap.Int("tool_result_count", len(toolResultMessages)),
						zap.Int("pending_tool_calls", len(toolCallAccumulator)))

					if assistantMessage != nil {
						iterationEvent := types.NewIterationCompletedEvent(iteration, "streaming-task", assistantMessage)
						select {
						case outputChan <- iterationEvent:
						case <-time.After(100 * time.Millisecond):
						}
					}

					cancelledStatusEvent := cloudevents.NewEvent()
					cancelledStatusEvent.SetType(types.EventTaskStatusChanged)
					if err := cancelledStatusEvent.SetData(cloudevents.ApplicationJSON, types.TaskStatus{
						State: types.TaskStateCanceled,
					}); err != nil {
						a.logger.Error("failed to set cancelled status event data", zap.Error(err))
						return
					}
					select {
					case outputChan <- cancelledStatusEvent:
					case <-time.After(100 * time.Millisecond):
					}

					interruptMessage := types.NewStreamingStatusMessage(
						fmt.Sprintf("task-interrupted-%d", iteration),
						string(types.TaskStateCanceled),
						nil,
					)
					interruptMessage.TaskID = taskID
					interruptMessage.ContextID = contextID
					select {
					case outputChan <- types.NewMessageEvent(types.EventTaskInterrupted, interruptMessage.MessageID, interruptMessage):
					default:
					}
					return

				case streamErr := <-streamErrorChan:
					if streamErr != nil {
						a.logger.Error("streaming failed", zap.Error(streamErr))

						failedStatusEvent := cloudevents.NewEvent()
						failedStatusEvent.SetType(types.EventTaskStatusChanged)
						if err := failedStatusEvent.SetData(cloudevents.ApplicationJSON, types.TaskStatus{
							State: types.TaskStateFailed,
						}); err != nil {
							a.logger.Error("failed to set failed status event data", zap.Error(err))
							return
						}
						select {
						case outputChan <- failedStatusEvent:
						default:
						}

						errorMessage := types.NewStreamingStatusMessage(
							fmt.Sprintf("streaming-error-%d", iteration),
							string(types.TaskStateFailed),
							nil,
						)
						errorMessage.TaskID = taskID
						errorMessage.ContextID = contextID
						select {
						case outputChan <- types.NewMessageEvent(types.EventStreamFailed, errorMessage.MessageID, errorMessage):
						default:
						}
						return
					}
					streaming = false

				case streamResp, ok := <-streamResponseChan:
					if !ok {
						streaming = false
						break
					}

					if streamResp == nil || len(streamResp.Choices) == 0 {
						continue
					}

					choice := streamResp.Choices[0]

					if streamResp.Usage != nil {
						usageTracker.AddTokenUsage(*streamResp.Usage)
					}

					if choice.Delta.Content != "" {
						fullContent += choice.Delta.Content

						chunkMessage := types.NewAssistantMessage(
							fmt.Sprintf("chunk-%d-%d", iteration, len(fullContent)),
							[]types.Part{types.NewTextPart(choice.Delta.Content)},
						)

						select {
						case outputChan <- types.NewDeltaEvent(chunkMessage):
						case <-ctx.Done():
							return
						}
					}

					for _, toolCallChunk := range choice.Delta.ToolCalls {
						key := fmt.Sprintf("%d", toolCallChunk.Index)

						if toolCallAccumulator[key] == nil {
							toolCallAccumulator[key] = &sdk.ChatCompletionMessageToolCall{
								Type:     "function",
								Function: sdk.ChatCompletionMessageToolCallFunction{},
							}
						}

						toolCall := toolCallAccumulator[key]
						if toolCallChunk.ID != "" {
							toolCall.Id = toolCallChunk.ID
						}
						if toolCallChunk.Function.Name != "" {
							toolCall.Function.Name = toolCallChunk.Function.Name
						}
						if toolCallChunk.Function.Arguments != "" {
							if toolCall.Function.Arguments == "" {
								toolCall.Function.Arguments = toolCallChunk.Function.Arguments
							} else if !isCompleteJSON(toolCall.Function.Arguments) {
								toolCall.Function.Arguments += toolCallChunk.Function.Arguments
							}
						}
					}

					if choice.FinishReason != "" {
						assistantMessage = types.NewAssistantMessage(
							fmt.Sprintf("assistant-stream-%d", iteration),
							make([]types.Part, 0),
						)
						assistantMessage.TaskID = taskID
						assistantMessage.ContextID = contextID

						if fullContent != "" {
							assistantMessage.Parts = append(assistantMessage.Parts, map[string]any{
								"kind": "text",
								"text": fullContent,
							})
						}

						llmResponse := &LLMResponse{Content: assistantMessage}
						if modified := executor.ExecuteAfterModel(ctx, callbackCtx, llmResponse); modified != nil && modified.Content != nil {
							assistantMessage = modified.Content
							assistantMessage.TaskID = taskID
							assistantMessage.ContextID = contextID
							for _, part := range assistantMessage.Parts {
								if partMap, ok := part.(map[string]any); ok {
									if text, exists := partMap["text"].(string); exists {
										fullContent = text
										break
									}
								}
							}
						}

						if len(toolCallAccumulator) > 0 && a.toolBox != nil {
							for key, toolCall := range toolCallAccumulator {
								a.logger.Debug("tool call accumulator",
									zap.String("key", key),
									zap.String("id", toolCall.Id),
									zap.String("name", toolCall.Function.Name),
									zap.String("arguments", toolCall.Function.Arguments))
							}

							toolCalls := make([]sdk.ChatCompletionMessageToolCall, 0, len(toolCallAccumulator))
							for _, toolCall := range toolCallAccumulator {
								toolCalls = append(toolCalls, *toolCall)
							}

							assistantMessage.Parts = append(assistantMessage.Parts, map[string]any{
								"kind": "data",
								"data": map[string]any{
									"tool_calls": toolCalls,
								},
							})

							currentMessages = append(currentMessages, *assistantMessage)
							iterationEvent := types.NewIterationCompletedEvent(iteration, "streaming-task", assistantMessage)
							select {
							case outputChan <- iterationEvent:
							case <-ctx.Done():
								return
							}

							toolResultMessages = a.executeToolCallsWithEvents(ctx, toolCalls, outputChan, usageTracker)

							for _, toolResult := range toolResultMessages {
								for _, part := range toolResult.Parts {
									if partMap, ok := part.(map[string]any); ok {
										if dataMap, exists := partMap["data"].(map[string]any); exists {
											if toolCallID, idExists := dataMap["tool_call_id"].(string); idExists {
												toolResults[toolCallID] = &toolResult
												break
											}
										}
									}
								}
							}
						} else {
							currentMessages = append(currentMessages, *assistantMessage)
							iterationEvent := types.NewIterationCompletedEvent(iteration, "streaming-task", assistantMessage)
							select {
							case outputChan <- iterationEvent:
							case <-ctx.Done():
								return
							}
						}
						streaming = false
					}
				}
			}

			if len(toolResultMessages) > 0 {
				currentMessages = append(currentMessages, toolResultMessages...)
				usageTracker.AddMessages(len(toolResultMessages))
				a.logger.Debug("persisted tool result messages",
					zap.Int("iteration", iteration),
					zap.Int("tool_result_count", len(toolResultMessages)))
			}

			if len(toolResultMessages) > 0 {
				lastToolMessage := toolResultMessages[len(toolResultMessages)-1]
				if lastToolMessage.Kind == "input_required" {
					a.logger.Debug("streaming completed - input required from user",
						zap.Int("iteration", iteration),
						zap.Int("final_message_count", len(currentMessages)))
					return
				}
			}

			if assistantMessage != nil && len(toolResultMessages) == 0 {
				a.logger.Debug("streaming completed - no tool calls executed",
					zap.Int("iteration", iteration),
					zap.Int("final_message_count", len(currentMessages)),
					zap.Bool("has_assistant_message", assistantMessage != nil))

				finalAssistantMessage = assistantMessage

				if modified := executor.ExecuteAfterAgent(ctx, callbackCtx, finalAssistantMessage); modified != nil {
					finalAssistantMessage = modified
					finalAssistantMessage.TaskID = taskID
					finalAssistantMessage.ContextID = contextID
				}

				completedStatusEvent := cloudevents.NewEvent()
				completedStatusEvent.SetType(types.EventTaskStatusChanged)
				if err := completedStatusEvent.SetData(cloudevents.ApplicationJSON, types.TaskStatus{
					State:   types.TaskStateCompleted,
					Message: finalAssistantMessage,
				}); err != nil {
					a.logger.Error("failed to set completed status event data", zap.Error(err))
					return
				}
				select {
				case outputChan <- completedStatusEvent:
				case <-time.After(100 * time.Millisecond):
				}

				return
			}

			a.logger.Debug("tool calls executed, continuing to next iteration",
				zap.Int("iteration", iteration),
				zap.Int("message_count", len(currentMessages)),
				zap.Int("tool_results_count", len(toolResultMessages)),
				zap.Int("unique_tool_calls", len(toolResults)))
		}

		a.logger.Warn("max streaming iterations reached", zap.Int("max_iterations", a.config.MaxChatCompletionIterations))

		canceledStatusEvent := cloudevents.NewEvent()
		canceledStatusEvent.SetType(types.EventTaskStatusChanged)
		if err := canceledStatusEvent.SetData(cloudevents.ApplicationJSON, types.TaskStatus{
			State: types.TaskStateCanceled,
		}); err != nil {
			a.logger.Error("failed to set canceled status event data", zap.Error(err))
			return
		}
		select {
		case outputChan <- canceledStatusEvent:
		case <-time.After(100 * time.Millisecond):
		}

		interruptMessage := types.NewStreamingStatusMessage(
			"max-iterations-reached",
			string(types.TaskStateCanceled),
			nil,
		)
		interruptMessage.TaskID = taskID
		interruptMessage.ContextID = contextID
		select {
		case outputChan <- types.NewMessageEvent(types.EventTaskInterrupted, interruptMessage.MessageID, interruptMessage):
		default:
		}
	}()

	return outputChan, nil
}

// executeToolCallsWithEvents executes tool calls and emits events, returning tool result messages
func (a *OpenAICompatibleAgentImpl) executeToolCallsWithEvents(ctx context.Context, toolCalls []sdk.ChatCompletionMessageToolCall, outputChan chan<- cloudevents.Event, usageTracker *UsageTracker) []types.Message {
	toolResultMessages := make([]types.Message, 0, len(toolCalls))

	var taskID *string
	var contextID *string
	if task, ok := ctx.Value(TaskContextKey).(*types.Task); ok && task != nil {
		taskID = &task.ID
		contextID = &task.ContextID
	}

	executor := a.GetCallbackExecutor()

	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "" {
			continue
		}

		usageTracker.IncrementToolCalls()

		toolStartMessage := types.NewStreamingStatusMessage(fmt.Sprintf("tool-start-%s", toolCall.Id), string(types.TaskStateWorking), nil)
		toolStartMessage.TaskID = taskID
		toolStartMessage.ContextID = contextID
		select {
		case outputChan <- types.NewMessageEvent(types.EventToolStarted, fmt.Sprintf("tool-start-%s", toolCall.Id), toolStartMessage):
		case <-ctx.Done():
			return toolResultMessages
		}

		var args map[string]any
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			a.logger.Error("failed to parse tool arguments", zap.String("tool", toolCall.Function.Name), zap.Error(err))
			usageTracker.IncrementFailedTools()

			toolFailedMessage := types.NewStreamingStatusMessage(fmt.Sprintf("tool-failed-%s", toolCall.Id), string(types.TaskStateFailed), nil)
			toolFailedMessage.TaskID = taskID
			toolFailedMessage.ContextID = contextID
			select {
			case outputChan <- types.NewMessageEvent(types.EventToolFailed, fmt.Sprintf("tool-failed-%s", toolCall.Id), toolFailedMessage):
			case <-ctx.Done():
			}

			toolResultMsg := types.NewToolResultMessage(toolCall.Id, toolCall.Function.Name, fmt.Sprintf("Error parsing tool arguments: %s", err.Error()), true)
			toolResultMsg.TaskID = taskID
			toolResultMsg.ContextID = contextID
			toolResultMessages = append(toolResultMessages, *toolResultMsg)
			continue
		}

		toolCtx := a.createToolContext(taskID, contextID)

		var tool Tool
		if a.toolBox != nil {
			tool, _ = a.toolBox.GetTool(toolCall.Function.Name)
		}

		switch toolCall.Function.Name {
		case types.ToolInputRequired:
			a.logger.Debug("input_required tool called in streaming mode", zap.String("tool_call_id", toolCall.Id), zap.String("message", toolCall.Function.Arguments))
			inputRequiredMessage := types.NewInputRequiredMessage(toolCall.Id, args["message"].(string))
			inputRequiredMessage.TaskID = taskID
			inputRequiredMessage.ContextID = contextID

			toolCompletedMessage := types.NewStreamingStatusMessage(fmt.Sprintf("tool-completed-%s", toolCall.Id), string(types.TaskStateCompleted), nil)
			toolCompletedMessage.TaskID = taskID
			toolCompletedMessage.ContextID = contextID
			select {
			case outputChan <- types.NewMessageEvent(types.EventToolCompleted, fmt.Sprintf("tool-completed-%s", toolCall.Id), toolCompletedMessage):
			case <-ctx.Done():
				return toolResultMessages
			}

			select {
			case outputChan <- types.NewMessageEvent(types.EventInputRequired, inputRequiredMessage.MessageID, inputRequiredMessage):
			case <-ctx.Done():
			}

			return append(toolResultMessages, *inputRequiredMessage)

		default:
			var result string
			var toolErr error

			if override := executor.ExecuteBeforeTool(ctx, tool, args, toolCtx); override != nil {
				a.logger.Debug("BeforeTool callback returned override, skipping tool execution",
					zap.String("tool", toolCall.Function.Name))
				if resultStr, ok := override["result"].(string); ok {
					result = resultStr
				} else {
					if jsonBytes, err := json.Marshal(override); err == nil {
						result = string(jsonBytes)
					}
				}
			} else {
				result, toolErr = a.toolBox.ExecuteTool(ctx, toolCall.Function.Name, args)
			}

			toolResult := map[string]interface{}{"result": result}
			if toolErr != nil {
				toolResult["error"] = toolErr.Error()
			}
			if modified := executor.ExecuteAfterTool(ctx, tool, args, toolCtx, toolResult); modified != nil {
				if resultStr, ok := modified["result"].(string); ok {
					result = resultStr
				}
				if _, hasError := modified["error"]; !hasError {
					toolErr = nil
				}
			}

			if toolErr != nil {
				a.logger.Error("failed to execute tool", zap.String("tool", toolCall.Function.Name), zap.Error(toolErr))
				usageTracker.IncrementFailedTools()
				result = fmt.Sprintf("Tool execution failed: %s", toolErr.Error())

				toolFailedMsg := types.NewStreamingStatusMessage(fmt.Sprintf("tool-failed-%s", toolCall.Id), string(types.TaskStateFailed), nil)
				toolFailedMsg.TaskID = taskID
				toolFailedMsg.ContextID = contextID
				select {
				case outputChan <- types.NewMessageEvent(types.EventToolFailed, fmt.Sprintf("tool-failed-%s", toolCall.Id), toolFailedMsg):
				case <-ctx.Done():
				}
			} else {
				toolCompletedMsg := types.NewStreamingStatusMessage(fmt.Sprintf("tool-completed-%s", toolCall.Id), string(types.TaskStateCompleted), nil)
				toolCompletedMsg.TaskID = taskID
				toolCompletedMsg.ContextID = contextID
				select {
				case outputChan <- types.NewMessageEvent(types.EventToolCompleted, fmt.Sprintf("tool-completed-%s", toolCall.Id), toolCompletedMsg):
				case <-ctx.Done():
					return toolResultMessages
				}
			}

			toolResultMessage := types.NewToolResultMessage(toolCall.Id, toolCall.Function.Name, result, toolErr != nil)
			toolResultMessage.TaskID = taskID
			toolResultMessage.ContextID = contextID
			select {
			case outputChan <- types.NewMessageEvent(types.EventToolResult, toolResultMessage.MessageID, toolResultMessage):
			case <-ctx.Done():
				return toolResultMessages
			}

			toolResultMessages = append(toolResultMessages, *toolResultMessage)
		}
	}

	return toolResultMessages
}

// isCompleteJSON checks if a string contains complete JSON by counting balanced braces
func isCompleteJSON(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return false
	}

	openCount := 0
	for _, char := range s {
		switch char {
		case '{':
			openCount++
		case '}':
			openCount--
		}
	}

	return openCount == 0
}

// createCallbackContext creates a CallbackContext from the current execution state
func (a *OpenAICompatibleAgentImpl) createCallbackContext(taskID, contextID *string) *CallbackContext {
	agentName := ""
	if a.config != nil {
		agentName = a.config.AgentName
	}

	callbackCtx := &CallbackContext{
		AgentName: agentName,
		State:     make(map[string]any),
		Logger:    a.logger,
	}

	if taskID != nil {
		callbackCtx.TaskID = *taskID
	}
	if contextID != nil {
		callbackCtx.ContextID = *contextID
	}

	return callbackCtx
}

// createToolContext creates a ToolContext from the current execution state
func (a *OpenAICompatibleAgentImpl) createToolContext(taskID, contextID *string) *ToolContext {
	agentName := ""
	if a.config != nil {
		agentName = a.config.AgentName
	}

	toolCtx := &ToolContext{
		AgentName: agentName,
		State:     make(map[string]any),
		Logger:    a.logger,
	}

	if taskID != nil {
		toolCtx.TaskID = *taskID
	}
	if contextID != nil {
		toolCtx.ContextID = *contextID
	}

	return toolCtx
}
