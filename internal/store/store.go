package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/turio/AmnesiaGraph/internal/graph"
)

const (
	StateDirName  = ".amnesiagraph"
	GraphsDirName = "graphs"
	CurrentName   = "current"

	// LegacyGraphFileName is the pre-named-graph single-file layout kept only
	// for one-case migration detection.
	LegacyGraphFileName = "graph.json"

	// DefaultGraphName receives legacy state during migration.
	DefaultGraphName = "default"
)

// ErrNotInitialized is returned when the current repository has no installed
// AmnesiaGraph state.
var ErrNotInitialized = errors.New("NOT_INITIALIZED")

// ErrMigrationConflict is returned when legacy single-file state and
// named-graph state coexist. No files are changed in that case.
var ErrMigrationConflict = errors.New("MIGRATION_CONFLICT")

// ErrNoCurrent is returned when no graph has been selected yet.
var ErrNoCurrent = errors.New("NO_CURRENT")

// ErrGraphNotFound is wrapped by Load when the named graph file does not
// exist, so callers can branch on absence with errors.Is without conflating
// malformed or unreadable graphs with missing ones.
var ErrGraphNotFound = errors.New("GRAPH_NOT_FOUND")

// GraphInfo exposes only what selection requires: storage name and file
// modification time. Completion stays a derived graph property.
type GraphInfo struct {
	Name    string
	ModTime time.Time
}

// ValidateGraphName enforces the portable storage-key rule:
// ^[a-z0-9][a-z0-9._-]*$
func ValidateGraphName(name string) error {
	if name == "" {
		return fmt.Errorf("INVALID_GRAPH_NAME %s", name)
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if i == 0 {
			if c < 'a' || c > 'z' {
				if c < '0' || c > '9' {
					return fmt.Errorf("INVALID_GRAPH_NAME %s", name)
				}
			}
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			continue
		}
		return fmt.Errorf("INVALID_GRAPH_NAME %s", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("INVALID_GRAPH_NAME %s", name)
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("INVALID_GRAPH_NAME %s", name)
	}
	return nil
}

// GraphsDir returns <root>/.amnesiagraph/graphs.
func GraphsDir(root string) string {
	return filepath.Join(root, StateDirName, GraphsDirName)
}

// CurrentPath returns <root>/.amnesiagraph/current.
func CurrentPath(root string) string {
	return filepath.Join(root, StateDirName, CurrentName)
}

// LegacyGraphPath returns the pre-named-graph single-file path, used only for
// migration detection.
func LegacyGraphPath(root string) string {
	return filepath.Join(root, StateDirName, LegacyGraphFileName)
}

// GraphPath returns the named graph file path. Callers must validate name
// with ValidateGraphName first; Load/Save do so automatically.
func GraphPath(root, name string) string {
	return filepath.Join(GraphsDir(root), name+".json")
}

// EnsureLayout creates .amnesiagraph/graphs/.
func EnsureLayout(root string) error {
	if err := os.MkdirAll(GraphsDir(root), 0o755); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	return nil
}

// Load reads a single named graph without consulting any other location.
func Load(root, name string) (graph.Graph, error) {
	if err := ValidateGraphName(name); err != nil {
		return graph.Graph{}, err
	}
	file, err := os.Open(GraphPath(root, name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return graph.Graph{}, fmt.Errorf("%w %s", ErrGraphNotFound, name)
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

// Save writes a single named graph through a temporary file and atomic rename.
func Save(root, name string, current graph.Graph) error {
	if err := ValidateGraphName(name); err != nil {
		return err
	}
	if err := EnsureLayout(root); err != nil {
		return err
	}
	directory := GraphsDir(root)

	temporary, err := os.CreateTemp(directory, name+"-*.tmp")
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
	if err := atomicRename(temporaryName, GraphPath(root, name)); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	keep = true
	return nil
}

// Exists reports whether the named graph file exists.
func Exists(root, name string) bool {
	if err := ValidateGraphName(name); err != nil {
		return false
	}
	_, err := os.Stat(GraphPath(root, name))
	return err == nil
}

// List returns named graphs with modification times. It does not read graph
// bodies and does not consult any other repository.
func List(root string) ([]GraphInfo, error) {
	entries, err := os.ReadDir(GraphsDir(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("LOAD_FAILED %w", err)
	}
	infos := make([]GraphInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		base := strings.TrimSuffix(name, ".json")
		if ValidateGraphName(base) != nil {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return nil, fmt.Errorf("LOAD_FAILED %w", statErr)
		}
		infos = append(infos, GraphInfo{Name: base, ModTime: info.ModTime()})
	}
	return infos, nil
}

// Current reads the selected graph name from .amnesiagraph/current.
func Current(root string) (string, error) {
	data, err := os.ReadFile(CurrentPath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoCurrent
		}
		return "", fmt.Errorf("LOAD_FAILED %w", err)
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", ErrNoCurrent
	}
	return name, nil
}

// SetCurrent atomically selects a named graph. It validates the name but does
// not require the target to exist; callers decide whether missing targets are
// an error or a fallback trigger.
func SetCurrent(root, name string) error {
	if err := ValidateGraphName(name); err != nil {
		return err
	}
	directory := filepath.Join(root, StateDirName)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	temporary, err := os.CreateTemp(directory, CurrentName+"-*.tmp")
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
	if _, err := temporary.WriteString(name + "\n"); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	if err := atomicRename(temporaryName, CurrentPath(root)); err != nil {
		return fmt.Errorf("SAVE_FAILED %w", err)
	}
	keep = true
	return nil
}

// hasNamedLayout reports whether any named-graph state has been established:
// at least one graphs/*.json file or a current pointer exists.
func hasNamedLayout(root string) bool {
	if _, err := os.Stat(CurrentPath(root)); err == nil {
		return true
	}
	infos, err := List(root)
	if err != nil || len(infos) > 0 {
		return len(infos) > 0
	}
	if _, err := os.Stat(GraphsDir(root)); err == nil {
		// Graphs dir exists but empty and no current pointer: not yet a layout
		// that could conflict with legacy state.
		return false
	}
	return false
}

func legacyExists(root string) bool {
	info, err := os.Stat(LegacyGraphPath(root))
	return err == nil && !info.IsDir()
}

// MigrateLegacyIfNeeded implements the one-case compatibility bridge from
// .amnesiagraph/graph.json to .amnesiagraph/graphs/default.json.
//
//   - legacy exists and no named layout: move bytes to graphs/default.json,
//     write current=default.
//   - legacy and named layout coexist: MIGRATION_CONFLICT, no mutation.
//   - otherwise: no-op.
func MigrateLegacyIfNeeded(root string) error {
	if !legacyExists(root) {
		return nil
	}
	if !hasNamedLayout(root) {
		if err := EnsureLayout(root); err != nil {
			return err
		}
		target := GraphPath(root, DefaultGraphName)
		if err := atomicRename(LegacyGraphPath(root), target); err != nil {
			// Cross-device rename is not expected inside one repo, but fall
			// back to copy+remove to preserve bytes rather than fail.
			data, readErr := os.ReadFile(LegacyGraphPath(root))
			if readErr != nil {
				return fmt.Errorf("MIGRATION_FAILED %w", readErr)
			}
			if writeErr := os.WriteFile(target, data, 0o644); writeErr != nil {
				return fmt.Errorf("MIGRATION_FAILED %w", writeErr)
			}
			_ = os.Remove(LegacyGraphPath(root))
		}
		if err := SetCurrent(root, DefaultGraphName); err != nil {
			return err
		}
		return nil
	}
	return ErrMigrationConflict
}
