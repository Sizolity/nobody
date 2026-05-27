package ingest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Source loading ---

func TestLoadSourceTxt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "story.txt")
	content := "Chapter 1\n\nThe hero arrived at the village.\n\nChapter 2\n\nA storm gathered over the mountains."
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	doc, err := LoadSource(path)
	require.NoError(t, err)
	assert.Equal(t, "story.txt", doc.Filename)
	assert.Equal(t, "txt", doc.Format)
	assert.Equal(t, content, doc.Text)
	assert.NotEmpty(t, doc.ID)
}

func TestLoadSourceMd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "world.md")
	content := "# Chapter 1\n\nThe hero arrived.\n\n# Chapter 2\n\nA storm gathered."
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	doc, err := LoadSource(path)
	require.NoError(t, err)
	assert.Equal(t, "world.md", doc.Filename)
	assert.Equal(t, "md", doc.Format)
	assert.Equal(t, content, doc.Text)
}

func TestLoadSourceUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.pdf")
	require.NoError(t, os.WriteFile(path, []byte("pdf data"), 0o644))

	_, err := LoadSource(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestLoadSourceNotFound(t *testing.T) {
	_, err := LoadSource("/nonexistent/path.txt")
	assert.Error(t, err)
}

// --- Chunker ---

func TestChunkTxt(t *testing.T) {
	doc := SourceDocument{
		ID:       "src_1",
		Filename: "story.txt",
		Format:   "txt",
		Text:     "Chapter 1\n\nThe hero arrived at the village.\n\nChapter 2\n\nA storm gathered over the mountains.",
	}
	chunks := Chunk(doc)
	require.True(t, len(chunks) >= 2, "expected at least 2 chunks, got %d", len(chunks))
	for _, c := range chunks {
		assert.Equal(t, "src_1", c.SourceID)
		assert.NotEmpty(t, c.ID)
		assert.NotEmpty(t, c.Text)
	}
}

func TestChunkMd(t *testing.T) {
	doc := SourceDocument{
		ID:       "src_2",
		Filename: "world.md",
		Format:   "md",
		Text:     "# Chapter 1\n\nThe hero arrived.\n\n# Chapter 2\n\nA storm gathered.",
	}
	chunks := Chunk(doc)
	require.True(t, len(chunks) >= 2, "expected at least 2 chunks, got %d", len(chunks))
	assert.Contains(t, chunks[0].Heading, "Chapter 1")
	assert.Contains(t, chunks[1].Heading, "Chapter 2")
}

func TestChunkEmptyDoc(t *testing.T) {
	doc := SourceDocument{ID: "src_empty", Format: "txt", Text: ""}
	chunks := Chunk(doc)
	assert.Empty(t, chunks)
}

// --- Parser interface with fake ---

type fakeParser struct {
	draft Draft
	err   error
}

func (f *fakeParser) Parse(_ context.Context, _ SourceDocument) (Draft, error) {
	return f.draft, f.err
}

func TestParserInterfaceFake(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "Kael", Confidence: 0.9, SourceRefs: []string{"ch1_p1"}},
		},
	}
	p := &fakeParser{draft: draft}
	doc := SourceDocument{ID: "src_1", Format: "txt", Text: "some text"}
	result, err := p.Parse(context.Background(), doc)
	require.NoError(t, err)
	require.Len(t, result.Entities, 1)
	assert.Equal(t, "char_hero", result.Entities[0].ID)
}

// --- Draft validation ---

func TestValidateDraftValid(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "Kael"},
		},
		Facts: []DraftFact{
			{ID: "fact_origin", SubjectID: "char_hero", Predicate: "origin", Value: "unknown"},
		},
	}
	report := ValidateDraft(draft)
	assert.Empty(t, report.Errors)
}

func TestValidateDraftInvalidID(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "", Type: "character", Name: "Nobody"},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors)
}

func TestValidateDraftMissingEntityName(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_x", Type: "character", Name: ""},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors)
}

func TestValidateDraftDanglingFactRef(t *testing.T) {
	draft := Draft{
		Facts: []DraftFact{
			{ID: "fact_1", SubjectID: "nonexistent_entity", Predicate: "status", Value: "alive"},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Warnings)
}

func TestValidateDraftDuplicateIDs(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_a", Type: "character", Name: "A"},
			{ID: "char_a", Type: "character", Name: "B"},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors)
}

// --- Compile draft into world ---

func TestCompileDraftNewWorld(t *testing.T) {
	world := model.World{
		ID:       "world_test",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	draft := Draft{
		Canon: &DraftCanon{
			Genre: []string{"fantasy"},
			Tone:  []string{"epic"},
		},
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "Kael", Description: "A wanderer."},
			{ID: "loc_village", Type: "location", Name: "Thornhaven", Description: "A quiet village."},
		},
		Relations: []DraftRelation{
			{ID: "rel_lives", Type: "lives_in", SourceID: "char_hero", TargetID: "loc_village"},
		},
		Facts: []DraftFact{
			{ID: "fact_origin", SubjectID: "char_hero", Predicate: "origin", Value: "unknown"},
		},
		Threads: []DraftThread{
			{ID: "thread_quest", Kind: "quest", Title: "Find the sword", Status: "open"},
		},
	}

	result, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, report.Inserted)
	assert.Equal(t, 0, report.Skipped)

	assert.Contains(t, result.Entities, model.EntityID("char_hero"))
	assert.Contains(t, result.Entities, model.EntityID("loc_village"))
	assert.Len(t, result.Relations, 1)
	assert.Len(t, result.Facts, 1)
	assert.Len(t, result.Threads, 1)
	assert.Equal(t, []string{"fantasy"}, result.Canon.Genre)
}

func TestCompileDraftSkipsExistingByDefault(t *testing.T) {
	world := model.World{
		ID:   "world_test",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_hero": {ID: "char_hero", Type: "character", Name: "OldKael"},
		},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "NewKael"},
			{ID: "char_sage", Type: "character", Name: "Mirael"},
		},
	}

	result, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Inserted)
	assert.Equal(t, 1, report.Skipped)
	assert.Equal(t, "OldKael", result.Entities["char_hero"].Name)
	assert.Equal(t, "Mirael", result.Entities["char_sage"].Name)
}

func TestCompileDraftReplacePolicy(t *testing.T) {
	world := model.World{
		ID:   "world_test",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_hero": {ID: "char_hero", Type: "character", Name: "OldKael"},
		},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "NewKael"},
		},
	}

	result, report, err := CompileDraft(world, draft, CompileOptions{ConflictPolicy: ConflictPolicyReplace})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Inserted)
	assert.Equal(t, 0, report.Skipped)
	assert.Equal(t, "NewKael", result.Entities["char_hero"].Name)
}

func TestCompileDraftRelationDanglingRef(t *testing.T) {
	world := model.World{
		ID:       "world_test",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	draft := Draft{
		Relations: []DraftRelation{
			{ID: "rel_x", Type: "knows", SourceID: "nonexistent_a", TargetID: "nonexistent_b"},
		},
	}

	_, _, err := CompileDraft(world, draft, CompileOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dangling")
}

func TestCompileDraftAllowDanglingRefs(t *testing.T) {
	world := model.World{
		ID:       "world_test",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	draft := Draft{
		Relations: []DraftRelation{
			{ID: "rel_x", Type: "knows", SourceID: "nonexistent_a", TargetID: "nonexistent_b"},
		},
	}

	_, _, err := CompileDraft(world, draft, CompileOptions{AllowDanglingRefs: true})
	assert.NoError(t, err)
}

// --- Import orchestration ---

func TestImportFile(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "novel.txt")
	content := "Chapter 1\n\nThe hero set out."
	require.NoError(t, os.WriteFile(srcPath, []byte(content), 0o644))

	worldDir := filepath.Join(dir, "workspace")
	require.NoError(t, os.MkdirAll(worldDir, 0o755))

	world := model.World{
		ID:       "world_import",
		Name:     "Import Test",
		Entities: map[model.EntityID]model.Entity{},
	}

	parser := &fakeParser{
		draft: Draft{
			Entities: []DraftEntity{
				{ID: "char_hero", Type: "character", Name: "Hero", SourceRefs: []string{"ch1"}},
			},
		},
	}

	result, err := ImportFile(context.Background(), world, srcPath, parser, CompileOptions{})
	require.NoError(t, err)
	assert.Contains(t, result.World.Entities, model.EntityID("char_hero"))
	assert.Equal(t, 1, result.CompileReport.Inserted)
	assert.NotEmpty(t, result.SourceDocument.ID)
}

// --- Compile report provenance ---

func TestCompileReportTracksSourceRefs(t *testing.T) {
	world := model.World{
		ID:       "world_test",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_a", Type: "character", Name: "A", SourceRefs: []string{"ch1_p1", "ch2_p3"}},
		},
	}

	_, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	require.Len(t, report.Provenance, 1)
	assert.Equal(t, "char_a", report.Provenance[0].WorldID)
	assert.Equal(t, []string{"ch1_p1", "ch2_p3"}, report.Provenance[0].SourceRefs)
}

// --- Edge cases ---

func TestChunkLargeText(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "This is a paragraph of text."
	}
	doc := SourceDocument{ID: "src_big", Format: "txt", Text: strings.Join(lines, "\n\n")}
	chunks := Chunk(doc)
	assert.NotEmpty(t, chunks)
}

func TestCompileDraftEmptyDraft(t *testing.T) {
	world := model.World{ID: "world_test", Name: "Test", Entities: map[model.EntityID]model.Entity{}}
	result, report, err := CompileDraft(world, Draft{}, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, report.Inserted)
	assert.Equal(t, world.ID, result.ID)
}

func TestCompileDraftThreadValidation(t *testing.T) {
	world := model.World{ID: "world_test", Name: "Test", Entities: map[model.EntityID]model.Entity{}}
	draft := Draft{
		Threads: []DraftThread{
			{ID: "t1", Kind: "quest", Title: "Quest", Status: "open"},
		},
	}
	result, _, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	require.Len(t, result.Threads, 1)
	assert.Equal(t, model.ThreadStatusOpen, result.Threads[0].Status)
}

func TestCompileDraftFactMerge(t *testing.T) {
	world := model.World{
		ID:       "world_test",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
		Facts: []model.Fact{
			{ID: "fact_existing", SubjectID: "char_a", Predicate: "status", Value: model.Value{Kind: model.ValueKindString, Raw: "alive"}},
		},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_a", Type: "character", Name: "A"},
		},
		Facts: []DraftFact{
			{ID: "fact_existing", SubjectID: "char_a", Predicate: "status", Value: "dead"},
			{ID: "fact_new", SubjectID: "char_a", Predicate: "mood", Value: "happy"},
		},
	}

	result, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Skipped)
	assert.Len(t, result.Facts, 2)
}
