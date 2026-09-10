package collector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSentState_MissingFileIsEmptyNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	state, err := LoadSentState(path)
	require.NoError(t, err)
	assert.False(t, state.Sent("msg_anything"))
}

func TestSentState_MarkAndSave_RoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	state, err := LoadSentState(path)
	require.NoError(t, err)
	assert.False(t, state.Sent("msg_1"))

	state.MarkSent("msg_1")
	state.MarkSent("msg_2")
	require.NoError(t, state.Save(path))

	reloaded, err := LoadSentState(path)
	require.NoError(t, err)
	assert.True(t, reloaded.Sent("msg_1"))
	assert.True(t, reloaded.Sent("msg_2"))
	assert.False(t, reloaded.Sent("msg_3"))
}

func TestSentState_FiltersAlreadySentRows(t *testing.T) {
	state, err := LoadSentState(filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)
	state.MarkSent("msg_1")

	rows := []UsageRow{{MessageID: "msg_1"}, {MessageID: "msg_2"}}
	unsent := state.FilterUnsent(rows)
	require.Len(t, unsent, 1)
	assert.Equal(t, "msg_2", unsent[0].MessageID)
}

func TestLoadSentState_MalformedFileIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))
	_, err := LoadSentState(path)
	assert.Error(t, err)
}
