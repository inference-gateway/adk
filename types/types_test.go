package types

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestTaskWithHistoryLength(t *testing.T) {
	history := []Message{{MessageID: "1"}, {MessageID: "2"}, {MessageID: "3"}}
	tests := []struct {
		name          string
		historyLength *int
		want          []string
	}{
		{name: "unset keeps all", historyLength: nil, want: []string{"1", "2", "3"}},
		{name: "zero keeps none", historyLength: new(0), want: []string{}},
		{name: "keeps the most recent", historyLength: new(2), want: []string{"2", "3"}},
		{name: "larger than history keeps all", historyLength: new(5), want: []string{"1", "2", "3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{History: history}.WithHistoryLength(tt.historyLength)
			ids := []string{}
			for _, message := range task.History {
				ids = append(ids, message.MessageID)
			}
			assert.Equal(t, tt.want, ids)
		})
	}
}

func TestTaskApplyArtifactUpdate(t *testing.T) {
	chunk := func(text string) Artifact {
		return Artifact{ArtifactID: "a1", Parts: []Part{CreateTextPart(text)}}
	}
	tests := []struct {
		name    string
		updates []TaskArtifactUpdateEvent
		want    []Artifact
	}{
		{
			name:    "new artifact is added",
			updates: []TaskArtifactUpdateEvent{{Artifact: chunk("one")}},
			want:    []Artifact{chunk("one")},
		},
		{
			name:    "append adds parts to the same artifact",
			updates: []TaskArtifactUpdateEvent{{Artifact: chunk("one")}, {Artifact: chunk("two"), Append: new(true)}},
			want:    []Artifact{{ArtifactID: "a1", Parts: []Part{CreateTextPart("one"), CreateTextPart("two")}}},
		},
		{
			name:    "same id without append replaces",
			updates: []TaskArtifactUpdateEvent{{Artifact: chunk("one")}, {Artifact: chunk("two")}},
			want:    []Artifact{chunk("two")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{}
			for _, update := range tt.updates {
				task.ApplyArtifactUpdate(update)
			}
			assert.Equal(t, tt.want, task.Artifacts)
		})
	}
}

func TestTaskWithoutExtension(t *testing.T) {
	uri := "https://example.com/ext/usage/v1"
	metadata := Struct{uri + "/usage": 1, uri + "-other/key": 2, "plain": 3}
	task := Task{ID: "t1", Metadata: &metadata}

	got := task.WithoutExtension(uri)

	assert.Equal(t, Struct{uri + "-other/key": 2, "plain": 3}, *got.Metadata)
	assert.Len(t, *task.Metadata, 3, "the original task must keep the extension keys")
	assert.Nil(t, Task{ID: "t2"}.WithoutExtension(uri).Metadata)

	onlyExtension := Struct{uri + "/usage": 1}
	assert.Nil(t, Task{ID: "t3", Metadata: &onlyExtension}.WithoutExtension(uri).Metadata)
}
