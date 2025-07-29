package server_test

import (
	"testing"

	"github.com/inference-gateway/adk/server"
	"github.com/stretchr/testify/assert"
)

func TestGenerateTaskID(t *testing.T) {
	id1 := server.GenerateTaskID()
	assert.NotEmpty(t, id1)

	id2 := server.GenerateTaskID()
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)

	assert.GreaterOrEqual(t, len(id1), 30)
	assert.GreaterOrEqual(t, len(id2), 30)
}

func TestGenerateTaskID_Uniqueness(t *testing.T) {
	numIDs := 100
	ids := make(map[string]bool)

	for i := 0; i < numIDs; i++ {
		id := server.GenerateTaskID()
		assert.NotEmpty(t, id)
		assert.False(t, ids[id], "Generated duplicate ID: %s", id)
		ids[id] = true
	}

	assert.Len(t, ids, numIDs, "All generated IDs should be unique")
}

func TestGenerateTaskID_Format(t *testing.T) {
	id := server.GenerateTaskID()

	assert.Regexp(t, `^[a-f0-9-]+$`, id, "ID should contain only lowercase hex digits and hyphens")
	assert.Regexp(t, `^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`, id, "ID should match UUID v4 format")
}

func TestGenerateTaskID_ThreadSafety(t *testing.T) {
	numGoroutines := 50
	numIDsPerGoroutine := 10

	idsChan := make(chan string, numGoroutines*numIDsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numIDsPerGoroutine; j++ {
				id := server.GenerateTaskID()
				idsChan <- id
			}
		}()
	}

	allIDs := make(map[string]bool)
	for i := 0; i < numGoroutines*numIDsPerGoroutine; i++ {
		id := <-idsChan
		assert.NotEmpty(t, id)
		assert.False(t, allIDs[id], "Generated duplicate ID in concurrent test: %s", id)
		allIDs[id] = true
	}

	expectedTotal := numGoroutines * numIDsPerGoroutine
	assert.Len(t, allIDs, expectedTotal, "All concurrently generated IDs should be unique")
}
