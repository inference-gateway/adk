module github.com/inference-gateway/adk/examples/queue-storage-examples/in-memory/server

go 1.23

replace github.com/inference-gateway/adk => ../../../../

require (
	github.com/inference-gateway/adk v0.0.0-00010101000000-000000000000
	github.com/sethvargo/go-envconfig v1.1.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/redis/go-redis/v9 v9.7.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
)