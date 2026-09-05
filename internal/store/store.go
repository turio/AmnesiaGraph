package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/turio/AmnesiaGraph/internal/graph"
)

const (
	StateDirName  = ".amnesiagraph"
	GraphFileName = "graph.json"
)

// ErrNotInitialized is returned when the current repository has no installed
// AmnesiaGraph state.
var ErrNotInitialized = errors.New("NOT_INITIALIZED")

// GraphPath returns the only V1 persistent state path for root.
func GraphPath(root string) string {
	return filepath.Join(root, StateDirName, GraphFileName)
}

// Load reads repo-local graph state without consulting any other location.
func Load(root string) (graph.Graph, error) {
	file, err := os.Open(GraphPath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return graph.Graph{}, ErrNotInitialized
		}
		return graph.Graph{}, fmt.Errorf("LOAD_FAILED %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var current graph.Graph
	if err := decoder.Decode(&current); err != nil {
		return graph.Graph{}, fmt.Errorf("INVALID_GRAPH malformed JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return graph.Graph{}, fmt.Errorf("INVALID_GRAPH multiple JSON values")
		}
		return graph.Graph{}, fmt.Errorf("INVALID_GRAPH malformed JSON: %w", err)
	}
	return current, nil
}

// Save writes graph state through a temporary file and replacement rename.
func Save(root string, current graph.Graph) error {
	directory := filepath.Join(root, StateDirName)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}

	temporary, err := os.CreateTemp(directory, GraphFileName+"-*.tmp")
	if err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	temporaryName := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryName)
		}
	}()

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(current); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	if err := atomicRename(temporaryName, GraphPath(root)); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	keep = true
	return nil
}
