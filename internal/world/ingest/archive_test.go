package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveSaveAndLoadSource(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	doc := SourceDocument{
		ID:       "src_novel",
		Filename: "novel.txt",
		Format:   "txt",
		Text:     "Once upon a time...",
	}
	require.NoError(t, archive.SaveSource(doc))

	loaded, err := archive.LoadSource("src_novel")
	require.NoError(t, err)
	assert.Equal(t, doc.ID, loaded.ID)
	assert.Equal(t, doc.Text, loaded.Text)
}

func TestArchiveLoadSourceNotFound(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	_, err := archive.LoadSource("nonexistent")
	assert.Error(t, err)
}

func TestArchiveSaveAndLoadProvenance(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	provenance := []ProvenanceEntry{
		{WorldID: "char_hero", Kind: "entity", SourceRefs: []string{"ch1_p1"}},
		{WorldID: "fact_origin", Kind: "fact", SourceRefs: []string{"ch2_p3"}},
	}
	require.NoError(t, archive.SaveProvenance("src_novel", provenance))

	loaded, err := archive.LoadProvenance("src_novel")
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, "char_hero", loaded[0].WorldID)
	assert.Equal(t, []string{"ch2_p3"}, loaded[1].SourceRefs)
}

func TestArchiveLoadProvenanceNotFound(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	loaded, err := archive.LoadProvenance("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestArchiveListSources(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	require.NoError(t, archive.SaveSource(SourceDocument{ID: "src_a", Filename: "a.txt", Format: "txt", Text: "A"}))
	require.NoError(t, archive.SaveSource(SourceDocument{ID: "src_b", Filename: "b.md", Format: "md", Text: "B"}))

	ids, err := archive.ListSources()
	require.NoError(t, err)
	assert.Contains(t, ids, "src_a")
	assert.Contains(t, ids, "src_b")
}

func TestArchiveListSourcesEmptyDir(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	ids, err := archive.ListSources()
	require.NoError(t, err)
	assert.Empty(t, ids)
}
