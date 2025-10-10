module input-required-streaming-client

go 1.21

replace github.com/inference-gateway/adk => ../../../../

require (
	github.com/inference-gateway/adk v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.26.0
)

require (
	github.com/cloudevents/sdk-go/v2 v2.15.2 // indirect
	github.com/google/uuid v1.4.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
)