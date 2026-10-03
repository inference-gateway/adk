package main

import (
	"slices"
	"testing"
)

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

func TestEnumVarnames(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		enum     []any
		want     []any
	}{
		{
			name:     "screaming snake values lose the type prefix",
			typeName: "TaskState",
			enum:     []any{"TASK_STATE_WORKING", "TASK_STATE_AUTH_REQUIRED"},
			want:     []any{"TaskStateWorking", "TaskStateAuthRequired"},
		},
		{
			name:     "pascal case values are kept verbatim",
			typeName: "A2AMethod",
			enum:     []any{"SendMessage", "ListTaskPushNotificationConfigs"},
			want:     []any{"A2AMethodSendMessage", "A2AMethodListTaskPushNotificationConfigs"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enumVarnames(tt.typeName, tt.enum)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("enumVarnames(%q, %v) = %v, want %v", tt.typeName, tt.enum, got, tt.want)
			}
		})
	}
}
