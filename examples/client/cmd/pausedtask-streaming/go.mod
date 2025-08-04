module pausedtask-streaming

go 1.24

replace github.com/inference-gateway/adk => ../../../..

require (
	github.com/inference-gateway/adk v0.0.0-00010101000000-000000000000
	github.com/sethvargo/go-envconfig v1.3.0
	go.uber.org/zap v1.27.0
)

require go.uber.org/multierr v1.10.0 // indirect
