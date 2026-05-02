package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	RelayID            string    `json:"relayId,omitempty"`
	NodeID             string    `json:"nodeId,omitempty"`
	ZTNetworkID        string    `json:"ztNetworkId,omitempty"`
	ConfigVersion      int64     `json:"configVersion"`
	NFTApplied         bool      `json:"nftApplied"`
	RouteApplied       bool      `json:"routeApplied"`
	LastRegisterAt     time.Time `json:"lastRegisterAt,omitempty"`
	LastHeartbeatAt    time.Time `json:"lastHeartbeatAt,omitempty"`
	LastApplyMessage   string    `json:"lastApplyMessage,omitempty"`
	LastControllerSeen time.Time `json:"lastControllerSeen,omitempty"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (store *Store) Load() (State, error) {
	content, err := os.ReadFile(store.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{
				NFTApplied:   true,
				RouteApplied: true,
			}, nil
		}
		return State{}, fmt.Errorf("read state file %s: %w", store.path, err)
	}

	var current State
	if err := json.Unmarshal(content, &current); err != nil {
		return State{}, fmt.Errorf("decode state file %s: %w", store.path, err)
	}

	return current, nil
}

func (store *Store) Save(current State) error {
	current.UpdatedAt = time.Now().UTC()

	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	content, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	content = append(content, '\n')

	tempPath := store.path + ".tmp"
	if err := os.WriteFile(tempPath, content, 0o600); err != nil {
		return fmt.Errorf("write temporary state file: %w", err)
	}
	if err := os.Rename(tempPath, store.path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace state file: %w", err)
	}

	return nil
}
