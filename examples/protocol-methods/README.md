# Protocol Methods A2A Example

This example exercises every JSON-RPC method the ADK supports beyond the
common `SendMessage`, `SendStreamingMessage`, and `GetTask` trio. It serves as a
runnable companion to the snippets in the root [`README.md`](../../README.md).

## Methods demonstrated

| Method                             | Where in the client                    |
| ---------------------------------- | -------------------------------------- |
| `GetExtendedAgentCard`             | `demonstrateAuthenticatedExtendedCard` |
| `ListTasks` (with pagination)      | `demonstrateListTasks`                 |
| `CreateTaskPushNotificationConfig` | `demonstratePushNotificationConfig`    |
| `GetTaskPushNotificationConfig`    | `demonstratePushNotificationConfig`    |
| `ListTaskPushNotificationConfigs`  | `demonstratePushNotificationConfig`    |
| `DeleteTaskPushNotificationConfig` | `demonstratePushNotificationConfig`    |
| `CancelTask`                       | `demonstrateCancel`                    |
| `SubscribeToTask`                  | `demonstrateResubscribe`               |

## How it works

The server in `server/main.go` registers a `SlowEchoTaskHandler` that:

- Sleeps for a few seconds inside `HandleTask` so the client has time to call
  `CancelTask` while the task is still `WORKING`.
- Emits a small sequence of streaming delta events from `HandleStreamingTask`
  so the client can drop the original SSE connection and reattach with
  `SubscribeToTask`.

The client in `client/main.go` submits four tasks (to give `ListTasks`
something to page through), runs the push notification config round-trip
against the first task, cancels the second, then opens and resubscribes to a
streaming task.

## Running the Example

### Prerequisites

- Go 1.26 or later
- Docker and Docker Compose (optional)

### Option 1: Using Docker Compose

```bash
docker-compose up --build
```

The `client` container waits for the server health check and then runs the
walkthrough end-to-end.

### Option 2: Running Locally

In one terminal:

```bash
cd server
go mod tidy
go run main.go
```

In another terminal:

```bash
cd client
go mod tidy
go run main.go
```

## Configuration

| Variable      | Purpose                                                                                  | Default                         |
| ------------- | ---------------------------------------------------------------------------------------- | ------------------------------- |
| `SERVER_URL`  | A2A server base URL                                                                      | `http://localhost:8080`         |
| `WEBHOOK_URL` | URL passed to `CreateTaskPushNotificationConfig` (the URL does not need to be reachable) | `http://localhost:9000/webhook` |
| `ENVIRONMENT` | Controls log verbosity                                                                   | `development`                   |

## Expected Output

A successful client run prints something like:

```text
=== GetExtendedAgentCard ===
{ ...agent card json... }

=== ListTasks (with pagination) ===
Page 1 (offset=0, returned=2, total=4):
  - <task-id-1> [state=TASK_STATE_WORKING]
  - <task-id-2> [state=TASK_STATE_WORKING]
Page 2 (offset=2, returned=2, total=4):
  - <task-id-3> [state=TASK_STATE_WORKING]
  - <task-id-4> [state=TASK_STATE_WORKING]

=== Create/Get/ListTaskPushNotificationConfig(s) ===
set → registered webhook for task <task-id-1>
get → ...
list → ...

=== DeleteTaskPushNotificationConfig ===
delete → webhook for task <task-id-1> removed

=== CancelTask ===
cancelled task <task-id-2> → state=TASK_STATE_CANCELED

=== SubscribeToTask ===
dropped initial stream for task <task-id>; re-attaching...
  event 1: ...
  event 2: ...
  ...

All protocol method demonstrations completed.
```

## Files Structure

```text
protocol-methods/
├── README.md
├── docker-compose.yaml
├── server/
│   ├── main.go
│   ├── config/config.go
│   ├── go.mod
│   └── go.sum
└── client/
    ├── main.go
    ├── go.mod
    └── go.sum

Note: Uses ../Dockerfile.server and ../Dockerfile.client for containers.
```
