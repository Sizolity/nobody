package ingest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SourceArchive persists source documents and their provenance alongside world data.
type SourceArchive struct {
	root string
}

// NewSourceArchive creates an archive rooted at the given directory.
// Typically this is <workspace>/worlds/<worldID>/sources/.
func NewSourceArchive(root string) *SourceArchive {
	return &SourceArchive{root: root}
}

// SaveSource persists a SourceDocument to disk.
func (a *SourceArchive) SaveSource(doc SourceDocument) error {
	dir := filepath.Join(a.root, doc.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(dir, "source.json"), doc)
}

// LoadSource loads a previously archived SourceDocument.
func (a *SourceArchive) LoadSource(sourceID string) (SourceDocument, error) {
	var doc SourceDocument
	path := filepath.Join(a.root, sourceID, "source.json")
	if err := readJSONFile(path, &doc); err != nil {
		return SourceDocument{}, fmt.Errorf("load source %q: %w", sourceID, err)
	}
	return doc, nil
}

// SaveProvenance persists the compile provenance report for a given source.
func (a *SourceArchive) SaveProvenance(sourceID string, provenance []ProvenanceEntry) error {
	dir := filepath.Join(a.root, sourceID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(dir, "provenance.json"), provenance)
}

// LoadProvenance loads the compile provenance report for a given source.
func (a *SourceArchive) LoadProvenance(sourceID string) ([]ProvenanceEntry, error) {
	var entries []ProvenanceEntry
	path := filepath.Join(a.root, sourceID, "provenance.json")
	if err := readJSONFile(path, &entries); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("load provenance %q: %w", sourceID, err)
	}
	return entries, nil
}

// ListSources returns the IDs of all archived sources.
func (a *SourceArchive) ListSources() ([]string, error) {
	entries, err := os.ReadDir(a.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
