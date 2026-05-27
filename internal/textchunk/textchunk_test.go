package textchunk

import (
	"strings"
	"testing"

	"github.com/sizolity/nobody/internal/world/ingest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicMarkdown(t *testing.T) {
	doc := ingest.SourceDocument{
		ID:     "src_2",
		Format: "md",
		Text:   "# Chapter 1\n\nThe hero arrived.\n\n# Chapter 2\n\nA storm gathered.",
	}
	chunks := Basic().Chunk(doc)
	require.True(t, len(chunks) >= 2, "expected at least 2 md chunks, got %d", len(chunks))
	assert.Contains(t, chunks[0].Heading, "Chapter 1")
	assert.Contains(t, chunks[1].Heading, "Chapter 2")
}

func TestBasicPlainText(t *testing.T) {
	doc := ingest.SourceDocument{
		ID:     "src_1",
		Format: "txt",
		Text:   "Chapter 1\n\nThe hero arrived at the village.\n\nChapter 2\n\nA storm gathered over the mountains.",
	}
	chunks := Basic().Chunk(doc)
	require.True(t, len(chunks) >= 2, "expected at least 2 txt chunks, got %d", len(chunks))
	for _, c := range chunks {
		assert.Equal(t, "src_1", c.SourceID)
		assert.NotEmpty(t, c.ID)
		assert.NotEmpty(t, c.Text)
	}
}

func TestBasicEmptyDoc(t *testing.T) {
	doc := ingest.SourceDocument{ID: "src_empty", Format: "txt", Text: ""}
	assert.Empty(t, Basic().Chunk(doc))
}

func TestBasicLargeText(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "This is a paragraph of text."
	}
	doc := ingest.SourceDocument{ID: "src_big", Format: "txt", Text: strings.Join(lines, "\n\n")}
	chunks := Basic().Chunk(doc)
	assert.NotEmpty(t, chunks)
}

func TestBasicPropagatesSourceKind(t *testing.T) {
	doc := ingest.SourceDocument{
		ID:     "src_x",
		Format: "md",
		Kind:   "novel",
		Text:   "# C1\n\nbody",
	}
	chunks := Basic().Chunk(doc)
	require.NotEmpty(t, chunks)
	for _, c := range chunks {
		assert.Equal(t, "novel", c.SourceKind)
	}
}

func TestBasicUnknownFormatFallsBackToWholeDoc(t *testing.T) {
	doc := ingest.SourceDocument{
		ID:     "src_y",
		Format: "html",
		Text:   "<p>hello</p>",
	}
	chunks := Basic().Chunk(doc)
	require.Len(t, chunks, 1, "unknown format should produce a single whole-doc chunk")
	assert.Equal(t, "<p>hello</p>", chunks[0].Text)
}

// --- ByFormat multiplexer ---

type stubChunker struct {
	called bool
}

func (s *stubChunker) Chunk(doc ingest.SourceDocument) []ingest.SourceChunk {
	s.called = true
	return []ingest.SourceChunk{{ID: doc.ID + "_stub", SourceID: doc.ID, Text: "stub"}}
}

func TestByFormatRoutesToMatchingHandler(t *testing.T) {
	mdStub := &stubChunker{}
	pdfStub := &stubChunker{}
	mux := ByFormat{
		Handlers: map[string]ingest.Chunker{
			"md":  mdStub,
			"pdf": pdfStub,
		},
		Default: ingest.WholeDocumentChunker{},
	}

	mux.Chunk(ingest.SourceDocument{ID: "d1", Format: "md", Text: "x"})
	assert.True(t, mdStub.called)
	assert.False(t, pdfStub.called)
}

func TestByFormatFallsBackToDefault(t *testing.T) {
	mux := ByFormat{
		Handlers: map[string]ingest.Chunker{"md": &stubChunker{}},
		Default:  ingest.WholeDocumentChunker{},
	}
	chunks := mux.Chunk(ingest.SourceDocument{ID: "d2", Format: "txt", Text: "hello"})
	require.Len(t, chunks, 1)
	assert.Equal(t, "hello", chunks[0].Text)
}

func TestByFormatNoDefaultUsesWholeDoc(t *testing.T) {
	mux := ByFormat{Handlers: map[string]ingest.Chunker{}}
	chunks := mux.Chunk(ingest.SourceDocument{ID: "d3", Format: "txt", Text: "hi"})
	require.Len(t, chunks, 1)
}
