// Package ingest provides product-neutral infrastructure for loading external
// narrative sources (txt, md) into structured world data. It defines schemas
// for source documents, extraction drafts, and a Parser interface that external
// implementations (e.g. LLM-based extractors) can satisfy.
package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SourceDocument holds the raw content of an ingested file along with metadata.
type SourceDocument struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Format   string `json:"format"`
	Text     string `json:"text"`
}

// SourceChunk is a stable slice of a source document with structural hints.
type SourceChunk struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id"`
	Index    int    `json:"index"`
	Heading  string `json:"heading,omitempty"`
	Text     string `json:"text"`
}

var supportedFormats = map[string]bool{
	"txt": true,
	"md":  true,
}

// LoadSource reads a file from disk and produces a SourceDocument.
// Only .txt and .md formats are supported.
func LoadSource(path string) (SourceDocument, error) {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if !supportedFormats[ext] {
		return SourceDocument{}, fmt.Errorf("unsupported source format %q", ext)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SourceDocument{}, err
	}
	filename := filepath.Base(path)
	id := "src_" + strings.TrimSuffix(filename, filepath.Ext(filename))
	return SourceDocument{
		ID:       id,
		Filename: filename,
		Format:   ext,
		Text:     string(data),
	}, nil
}

// Chunk splits a SourceDocument into SourceChunks based on format heuristics.
// For markdown, splits on top-level headings. For plain text, splits on blank-line
// separated paragraphs, grouping small adjacent paragraphs.
func Chunk(doc SourceDocument) []SourceChunk {
	text := strings.TrimSpace(doc.Text)
	if text == "" {
		return nil
	}
	if doc.Format == "md" {
		return chunkMarkdown(doc, text)
	}
	return chunkPlainText(doc, text)
}

func chunkMarkdown(doc SourceDocument, text string) []SourceChunk {
	lines := strings.Split(text, "\n")
	var chunks []SourceChunk
	var current strings.Builder
	currentHeading := ""
	idx := 0

	flush := func() {
		body := strings.TrimSpace(current.String())
		if body == "" && currentHeading == "" {
			return
		}
		chunks = append(chunks, SourceChunk{
			ID:       fmt.Sprintf("%s_chunk_%d", doc.ID, idx),
			SourceID: doc.ID,
			Index:    idx,
			Heading:  currentHeading,
			Text:     body,
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

func chunkPlainText(doc SourceDocument, text string) []SourceChunk {
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

	var chunks []SourceChunk
	var current strings.Builder
	idx := 0

	flush := func() {
		body := strings.TrimSpace(current.String())
		if body == "" {
			return
		}
		chunks = append(chunks, SourceChunk{
			ID:       fmt.Sprintf("%s_chunk_%d", doc.ID, idx),
			SourceID: doc.ID,
			Index:    idx,
			Text:     body,
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
			chunks = append(chunks, SourceChunk{
				ID:       fmt.Sprintf("%s_chunk_%d", doc.ID, i),
				SourceID: doc.ID,
				Index:    i,
				Text:     para,
			})
		}
	}
	return chunks
}
