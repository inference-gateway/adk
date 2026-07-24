package main

import "testing"

// TestHoistTitledEnumsDeterministic guards the non-determinism bug that once
// broke CI: the same titled enum appears inline in two places with different
// descriptions, and map iteration must not decide which description wins.
func TestHoistTitledEnumsDeterministic(t *testing.T) {
	newSchemas := func() map[string]any {
		mkState := func(desc string) map[string]any {
			return map[string]any{
				"type":        "object",
				"description": desc,
				"properties": map[string]any{
					"state": map[string]any{
						"type":        "string",
						"title":       "Task State",
						"enum":        []any{"TASK_STATE_WORKING", "TASK_STATE_COMPLETED"},
						"description": desc,
					},
				},
			}
		}
		return map[string]any{
			"AListTasksRequest": mkState("first alphabetically"),
			"TaskStatus":        mkState("last alphabetically"),
		}
	}

	// Alphabetically-first key always wins, regardless of map iteration order.
	const want = "first alphabetically"
	for i := 0; i < 50; i++ {
		schemas := newSchemas()
		hoisted := map[string]any{}
		hoistTitledEnums(hoisted, schemas)
		got := hoisted["TaskState"].(map[string]any)["description"]
		if got != want {
			t.Fatalf("run %d: TaskState description = %q, want %q", i, got, want)
		}
	}
}
