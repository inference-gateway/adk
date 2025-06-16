package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/inference-gateway/a2a/adk"
	"github.com/inference-gateway/a2a/adk/client"
	"go.uber.org/zap"
)

func main() {
	// Create a logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}

	// Create a client - replace with your actual A2A server URL
	a2aClient := client.NewClientWithLogger("http://localhost:8080", logger)

	// Test ListTasks with various parameters
	ctx := context.Background()

	// Test 1: List all tasks with default pagination
	fmt.Println("=== Example 1: List all tasks (default pagination) ===")
	params1 := adk.TaskListParams{}
	resp1, err := a2aClient.ListTasks(ctx, params1)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		// Parse the response result as TaskList
		resultBytes, _ := json.Marshal(resp1.Result)
		var taskList adk.TaskList
		json.Unmarshal(resultBytes, &taskList)

		fmt.Printf("Found %d total tasks, showing %d (limit: %d, offset: %d)\n",
			taskList.Total, len(taskList.Tasks), taskList.Limit, taskList.Offset)

		for i, task := range taskList.Tasks {
			fmt.Printf("  Task %d: ID=%s, State=%s, Context=%s\n",
				i+1, task.ID, task.Status.State, task.ContextID)
		}
	}

	// Test 2: List tasks with custom limit and offset
	fmt.Println("\n=== Example 2: List tasks with pagination (limit=5, offset=0) ===")
	params2 := adk.TaskListParams{
		Limit:  5,
		Offset: 0,
	}
	resp2, err := a2aClient.ListTasks(ctx, params2)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		resultBytes, _ := json.Marshal(resp2.Result)
		var taskList adk.TaskList
		json.Unmarshal(resultBytes, &taskList)

		fmt.Printf("Found %d total tasks, showing %d (limit: %d, offset: %d)\n",
			taskList.Total, len(taskList.Tasks), taskList.Limit, taskList.Offset)
	}

	// Test 3: Filter by state
	fmt.Println("\n=== Example 3: Filter by completed state ===")
	completedState := adk.TaskStateCompleted
	params3 := adk.TaskListParams{
		State: &completedState,
		Limit: 10,
	}
	resp3, err := a2aClient.ListTasks(ctx, params3)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		resultBytes, _ := json.Marshal(resp3.Result)
		var taskList adk.TaskList
		json.Unmarshal(resultBytes, &taskList)

		fmt.Printf("Found %d completed tasks, showing %d\n",
			taskList.Total, len(taskList.Tasks))

		for i, task := range taskList.Tasks {
			fmt.Printf("  Task %d: ID=%s, State=%s\n",
				i+1, task.ID, task.Status.State)
		}
	}

	// Test 4: Filter by context ID
	fmt.Println("\n=== Example 4: Filter by context ID ===")
	contextID := "some-context-id"
	params4 := adk.TaskListParams{
		ContextID: &contextID,
		Limit:     20,
	}
	resp4, err := a2aClient.ListTasks(ctx, params4)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		resultBytes, _ := json.Marshal(resp4.Result)
		var taskList adk.TaskList
		json.Unmarshal(resultBytes, &taskList)

		fmt.Printf("Found %d tasks for context '%s', showing %d\n",
			taskList.Total, contextID, len(taskList.Tasks))
	}

	fmt.Println("\n=== All examples completed ===")
	fmt.Println("\nNOTE: This example will fail if no A2A server is running on localhost:8080.")
	fmt.Println("To test against a real server, update the URL in the code.")
}
