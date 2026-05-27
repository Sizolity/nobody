// Package textchunk provides text-format-specific implementations of the
// ingest.Chunker contract. It lives outside internal/world/ingest on purpose:
// the world package's job is the ingestion schema, not the text-splitting
// heuristics, and future format extensions (PDF, EPUB, HTML, subtitle files,
// structured world-spec YAML, ...) belong in this sibling package or new ones
// like it.
//
// Current implementations:
//   - Basic(): markdown h1 + plain-text blank-line paragraphs (legacy default
//     behavior preserved from earlier ingest.Chunk()).
//   - ByFormat: a multiplexer that picks an inner Chunker based on
//     SourceDocument.Format, so callers can register custom format handlers.
package textchunk

import (
	"fmt"
	"strings"

	"github.com/sizolity/nobody/internal/world/ingest"
)

// Basic returns an ingest.Chunker with the legacy heuristic behavior:
//   - md  → split on top-level "# " headings, one chunk per section
//   - txt → group blank-line-separated paragraphs up to 500 chars; if the
//     entire document fits in one merged chunk but contains multiple
//     paragraphs, fall back to one-chunk-per-paragraph for finer granularity
//   - any other format → fall back to WholeDocumentChunker
//
// This is a low-effort default suitable for short stories, novel excerpts,
// and hand-written setting docs. For large books or specialized formats,
// implement a custom Chunker.
func Basic() ingest.Chunker {
	return basicChunker{}
}

type basicChunker struct{}

func (basicChunker) Chunk(doc ingest.SourceDocument) []ingest.SourceChunk {
	text := strings.TrimSpace(doc.Text)
	if text == "" {
		return nil
	}
	switch doc.Format {
	case "md":
		return chunkMarkdown(doc, text)
	case "txt":
		return chunkPlainText(doc, text)
	default:
		return ingest.WholeDocumentChunker{}.Chunk(doc)
	}
}

// ByFormat lets callers register format-specific chunkers and falls back to
// a configurable Default when no handler matches.
//
//	mux := textchunk.ByFormat{
//	    "md":  textchunk.Basic(),
//	    "pdf": myPDFChunker,
//	}
//	mux.Default = ingest.WholeDocumentChunker{}
type ByFormat struct {
	Handlers map[string]ingest.Chunker
	Default  ingest.Chunker
}

// Chunk implements ingest.Chunker.
func (m ByFormat) Chunk(doc ingest.SourceDocument) []ingest.SourceChunk {
	if h, ok := m.Handlers[doc.Format]; ok {
		return h.Chunk(doc)
	}
	if m.Default != nil {
		return m.Default.Chunk(doc)
	}
	return ingest.WholeDocumentChunker{}.Chunk(doc)
}

func chunkMarkdown(doc ingest.SourceDocument, text string) []ingest.SourceChunk {
	lines := strings.Split(text, "\n")
	var chunks []ingest.SourceChunk
	var current strings.Builder
	currentHeading := ""
	idx := 0

	flush := func() {
		body := strings.TrimSpace(current.String())
		if body == "" && currentHeading == "" {
			return
		}
		chunks = append(chunks, ingest.SourceChunk{
			ID:         fmt.Sprintf("%s_chunk_%d", doc.ID, idx),
			SourceID:   doc.ID,
			SourceKind: doc.Kind,
			Index:      idx,
			Heading:    currentHeading,
			Text:       body,
		})
		idx++
		current.Reset()
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			flush()
			currentHeading = strings.TrimPrefix(line, "# ")
		} else {
			current.WriteString(line)
			current.WriteString("\n")
		}
	}
	flush()
	return chunks
}

func chunkPlainText(doc ingest.SourceDocument, text string) []ingest.SourceChunk {
	paragraphs := strings.Split(text, "\n\n")
	var nonEmpty []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return nil
	}

	var chunks []ingest.SourceChunk
	var current strings.Builder
	idx := 0

	flush := func() {
		body := strings.TrimSpace(current.String())
		if body == "" {
			return
		}
		chunks = append(chunks, ingest.SourceChunk{
			ID:         fmt.Sprintf("%s_chunk_%d", doc.ID, idx),
			SourceID:   doc.ID,
			SourceKind: doc.Kind,
			Index:      idx,
			Text:       body,
		})
		idx++
		current.Reset()
	}

	for _, para := range nonEmpty {
		if current.Len() > 0 && current.Len()+len(para) > 500 {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
	}
	flush()

	// If everything ended up in one chunk but there are multiple paragraphs,
	// split into individual paragraph chunks for better granularity.
	if len(chunks) == 1 && len(nonEmpty) > 1 {
		chunks = nil
		for i, para := range nonEmpty {
			chunks = append(chunks, ingest.SourceChunk{
				ID:         fmt.Sprintf("%s_chunk_%d", doc.ID, i),
				SourceID:   doc.ID,
				SourceKind: doc.Kind,
				Index:      i,
				Text:       para,
			})
		}
	}
	return chunks
}
