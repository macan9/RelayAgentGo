package state

import (
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefaultState(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "state.json"))

	current, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !current.NFTApplied || !current.RouteApplied {
		t.Fatalf("expected default applied flags to be true: %+v", current)
	}
}

func TestSaveAndLoadState(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "nested", "state.json"))

	expected := State{
		RelayID:       "relay-1",
		NodeID:        "node-1",
		ZTNetworkID:   "8056c2e21c000001",
		ConfigVersion: 12,
		NFTApplied:    false,
		RouteApplied:  false,
	}
	if err := store.Save(expected); err != nil {
		t.Fatalf("save state: %v", err)
	}

	actual, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if actual.RelayID != expected.RelayID || actual.NodeID != expected.NodeID || actual.ZTNetworkID != expected.ZTNetworkID || actual.ConfigVersion != expected.ConfigVersion {
		t.Fatalf("unexpected state: %+v", actual)
	}
	if actual.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set")
	}
}
