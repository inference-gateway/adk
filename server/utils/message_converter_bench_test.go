package utils

import (
	"testing"

	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

func BenchmarkMessageConverter_ConvertToSDK(b *testing.B) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	messages := []types.Message{
		{
			MessageID: "bench-msg-1",
			Role:      "user",
			Parts: []types.Part{
				types.CreateTextPart("This is a benchmark test message with some content to convert."),
			},
		},
		{
			MessageID: "bench-msg-2",
			Role:      "assistant",
			Parts: []types.Part{
				types.CreateTextPart("This is a response message from the assistant."),
			},
		},
		{
			MessageID: "bench-msg-3",
			Role:      "system",
			Parts: []types.Part{
				types.CreateTextPart("System message with instructions for the assistant."),
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := converter.ConvertToSDK(messages)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageConverter_ConvertFromSDK(b *testing.B) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	sdkMessage, err := sdk.NewTextMessage(sdk.Assistant, "This is a benchmark test response from the SDK with some content to convert back to A2A format.")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := converter.ConvertFromSDK(sdkMessage)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageConverter_ConvertToSDK_StronglyTyped(b *testing.B) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	messages := []types.Message{
		{
			MessageID: "bench-typed-msg-1",
			Role:      "user",
			Parts: []types.Part{
				types.CreateTextPart("This is a strongly-typed benchmark test message."),
			},
		},
		{
			MessageID: "bench-typed-msg-2",
			Role:      "assistant",
			Parts: []types.Part{
				types.CreateTextPart("This is a strongly-typed response message."),
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := converter.ConvertToSDK(messages)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageConverter_ConvertToSDK_LargeMessages(b *testing.B) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	largeContent := ""
	for i := 0; i < 1000; i++ {
		largeContent += "This is a large message content for benchmarking purposes. "
	}

	messages := []types.Message{
		{
			MessageID: "bench-large-msg",
			Role:      "user",
			Parts: []types.Part{
				types.CreateTextPart(largeContent),
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := converter.ConvertToSDK(messages)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageConverter_ConvertToSDK_ManyMessages(b *testing.B) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	messages := make([]types.Message, 100)
	for i := 0; i < 100; i++ {
		messages[i] = types.Message{
			MessageID: "bench-many-msg-" + string(rune(i)),
			Role:      "user",
			Parts: []types.Part{
				types.CreateTextPart("Message number " + string(rune(i))),
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := converter.ConvertToSDK(messages)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageConverter_ValidateMessagePart(b *testing.B) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	part := types.CreateTextPart("This is a test message part for validation benchmarking.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := converter.ValidateMessagePart(part)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageConverter_ValidateMessagePart_StronglyTyped(b *testing.B) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	part := types.CreateTextPart("This is a strongly-typed test message part for validation benchmarking.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := converter.ValidateMessagePart(part)
		if err != nil {
			b.Fatal(err)
		}
	}
}
