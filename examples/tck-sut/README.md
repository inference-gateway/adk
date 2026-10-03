# A2A TCK System Under Test

An ADK server that implements the [A2A TCK](https://github.com/a2aproject/a2a-tck) scenarios
(`scenarios/core_operations.feature` and `scenarios/streaming.feature`). The TCK selects each
scenario through the `messageId` prefix it sends (`tck-complete-task`, `tck-artifact-text`, ...).
CI runs the TCK JSON-RPC MUST-level suite against it on every pull request.

## Running the TCK locally

```bash
cd examples/tck-sut && go run .
```

In another terminal:

```bash
git clone https://github.com/a2aproject/a2a-tck && cd a2a-tck
uv run ./run_tck.py --sut-host http://localhost:9999 --transport jsonrpc --level must
```

## Known gaps

- `tests/compatibility/core_operations/test_artifacts.py` fails: `SendMessage` returns the task before the
  handler runs, and a handler cannot answer with a direct `Message`. CI deselects it.
- Streamed artifacts are stored on the task but not sent as `artifactUpdate` events; the ADK has no
  artifact-update event yet.
