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

	outputChan := make(chan cloudevents.Event, 100)

	go func() {
		defer close(outputChan)

		currentMessages := make([]types.Message, len(messages))
		copy(currentMessages, messages)

		for iteration := 1; iteration <= a.config.MaxChatCompletionIterations; iteration++ {
			a.logger.Debug("starting streaming iteration",
				zap.Int("iteration", iteration),
				zap.Int("message_count", len(currentMessages)))

			sdkMessages, err := a.converter.ConvertToSDK(currentMessages)
			if err != nil {
				a.logger.Error("failed to convert messages to SDK format", zap.Error(err))
				return
			}

			if a.config != nil && a.config.SystemPrompt != "" {
				systemMessage := sdk.Message{
					Role:    sdk.System,
					Content: a.config.SystemPrompt,
				}
				sdkMessages = append([]sdk.Message{systemMessage}, sdkMessages...)
			}

			streamResponseChan, streamErrorChan := a.llmClient.CreateStreamingChatCompletion(ctx, sdkMessages, tools...)

			var fullContent string
			toolCallAccumulator := make(map[string]*sdk.ChatCompletionMessageToolCall)
			var assistantMessage *types.Message
			var toolResultMessages []types.Message
			toolResults := make(map[string]*types.Message)

			streaming := true
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

					interruptMessage := types.NewStreamingStatusMessage(
						fmt.Sprintf("task-interrupted-%d", iteration),
						string(types.TaskStateCanceled),
						nil,
					)
					select {
					case outputChan <- types.NewMessageEvent(types.EventTaskInterrupted, interruptMessage.MessageID, interruptMessage):
					default:
					}
					return

				case streamErr := <-streamErrorChan:
					if streamErr != nil {
						a.logger.Error("streaming failed", zap.Error(streamErr))

						errorMessage := types.NewStreamingStatusMessage(
							fmt.Sprintf("streaming-error-%d", iteration),
							string(types.TaskStateFailed),
							nil,
						)
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

						if fullContent != "" {
							assistantMessage.Parts = append(assistantMessage.Parts, map[string]any{
								"kind": "text",
								"text": fullContent,
							})
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

							toolResultMessages = a.executeToolCallsWithEvents(ctx, toolCalls, outputChan)

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
				return
			}

			a.logger.Debug("tool calls executed, continuing to next iteration",
				zap.Int("iteration", iteration),
				zap.Int("message_count", len(currentMessages)),
				zap.Int("tool_results_count", len(toolResultMessages)),
				zap.Int("unique_tool_calls", len(toolResults)))
		}

		a.logger.Warn("max streaming iterations reached", zap.Int("max_iterations", a.config.MaxChatCompletionIterations))
	}()

	return outputChan, nil
}

// executeToolCallsWithEvents executes tool calls and emits events, returning tool result messages
func (a *OpenAICompatibleAgentImpl) executeToolCallsWithEvents(ctx context.Context, toolCalls []sdk.ChatCompletionMessageToolCall, outputChan chan<- cloudevents.Event) []types.Message {
	toolResultMessages := make([]types.Message, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "" {
			continue
		}

		select {
		case outputChan <- types.NewMessageEvent(types.EventToolStarted, fmt.Sprintf("tool-start-%s", toolCall.Id),
			types.NewStreamingStatusMessage(fmt.Sprintf("tool-start-%s", toolCall.Id), string(types.TaskStateWorking), nil)):
		case <-ctx.Done():
			return toolResultMessages
		}

		var args map[string]any
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			a.logger.Error("failed to parse tool arguments", zap.String("tool", toolCall.Function.Name), zap.Error(err))

			select {
			case outputChan <- types.NewMessageEvent(types.EventToolFailed, fmt.Sprintf("tool-failed-%s", toolCall.Id),
				types.NewStreamingStatusMessage(fmt.Sprintf("tool-failed-%s", toolCall.Id), string(types.TaskStateFailed), nil)):
			case <-ctx.Done():
			}

			toolResultMessages = append(toolResultMessages, *types.NewToolResultMessage(toolCall.Id, toolCall.Function.Name, fmt.Sprintf("Error parsing tool arguments: %s", err.Error()), true))
			continue
		}

		switch toolCall.Function.Name {
		case types.ToolInputRequired:
			a.logger.Debug("input_required tool called in streaming mode", zap.String("tool_call_id", toolCall.Id), zap.String("message", toolCall.Function.Arguments))
			inputRequiredMessage := types.NewInputRequiredMessage(toolCall.Id, args["message"].(string))

			select {
			case outputChan <- types.NewMessageEvent(types.EventToolCompleted, fmt.Sprintf("tool-completed-%s", toolCall.Id),
				types.NewStreamingStatusMessage(fmt.Sprintf("tool-completed-%s", toolCall.Id), string(types.TaskStateCompleted), nil)):
			case <-ctx.Done():
				return toolResultMessages
			}

			select {
			case outputChan <- types.NewMessageEvent(types.EventInputRequired, inputRequiredMessage.MessageID, inputRequiredMessage):
			case <-ctx.Done():
			}

			return append(toolResultMessages, *inputRequiredMessage)

		default:
			result, toolErr := a.toolBox.ExecuteTool(ctx, toolCall.Function.Name, args)

			if toolErr != nil {
				a.logger.Error("failed to execute tool", zap.String("tool", toolCall.Function.Name), zap.Error(toolErr))
				result = fmt.Sprintf("Tool execution failed: %s", toolErr.Error())

				select {
				case outputChan <- types.NewMessageEvent(types.EventToolFailed, fmt.Sprintf("tool-failed-%s", toolCall.Id),
					types.NewStreamingStatusMessage(fmt.Sprintf("tool-failed-%s", toolCall.Id), string(types.TaskStateFailed), nil)):
				case <-ctx.Done():
				}
			} else {
				select {
				case outputChan <- types.NewMessageEvent(types.EventToolCompleted, fmt.Sprintf("tool-completed-%s", toolCall.Id),
					types.NewStreamingStatusMessage(fmt.Sprintf("tool-completed-%s", toolCall.Id), string(types.TaskStateCompleted), nil)):
				case <-ctx.Done():
					return toolResultMessages
				}
			}

			toolResultMessage := types.NewToolResultMessage(toolCall.Id, toolCall.Function.Name, result, toolErr != nil)
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
