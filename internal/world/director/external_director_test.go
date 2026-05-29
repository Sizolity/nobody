package director

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestExternalDirectorNoPendingReturnsEmpty(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)
	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d events", len(got))
	}
}

func TestExternalDirectorSubmitEventsWithCorrectSource(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)
	d.SubmitEvents(
		model.WorldEvent{
			ID:          "event_test_1",
			Type:        model.EventTypeNote,
			Source:      "original_source",
			Description: "Test event",
		},
		model.WorldEvent{
			ID:          "event_test_2",
			Type:        model.EventTypeMove,
			Source:      "original_source",
			Description: "Move event",
		},
	)

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	for i, e := range got {
		if e.Source != model.EventSourceUser {
			t.Errorf("event[%d].Source = %q, want %q", i, e.Source, model.EventSourceUser)
		}
	}
}

func TestExternalDirectorSubmitEventsCustomSource(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)
	d.Submit(ExternalInput{
		Events: []model.WorldEvent{{
			ID:          "event_api_1",
			Type:        model.EventTypeNote,
			Description: "From API",
		}},
		Source: "api_v2",
	})

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if got[0].Source != "api_v2" {
		t.Fatalf("source = %q, want %q", got[0].Source, "api_v2")
	}
}

func TestExternalDirectorSubmitTextNoTranslator(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)
	d.SubmitText("The hero opens the gate.", "")

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].Type != model.EventTypeNote {
		t.Errorf("type = %q, want %q", got[0].Type, model.EventTypeNote)
	}
	if got[0].Description != "The hero opens the gate." {
		t.Errorf("description = %q", got[0].Description)
	}
	if got[0].Source != model.EventSourceUser {
		t.Errorf("source = %q, want %q", got[0].Source, model.EventSourceUser)
	}
}

func TestExternalDirectorSubmitTextWithTranslator(t *testing.T) {
	t.Parallel()

	translated := []model.WorldEvent{{
		ID:          "event_gate_opened",
		Type:        model.EventTypeWorldFactChanged,
		Source:      model.EventSourceUser,
		Description: "The hero opens the gate.",
	}}
	responseJSON, _ := json.Marshal(translated)

	gen := &capturingGenerator{}
	gen.lastSystem = ""
	gen.lastUser = ""

	translatorGen := fakeGenerator{response: string(responseJSON)}
	d := NewExternalDirector("ext_1", translatorGen)
	d.SubmitText("open the gate", "")

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].ID != "event_gate_opened" {
		t.Errorf("event ID = %q", got[0].ID)
	}
}

func TestExternalDirectorTranslatorReceivesCorrectPrompts(t *testing.T) {
	t.Parallel()

	translated := []model.WorldEvent{{
		ID:     "event_ok",
		Type:   model.EventTypeNote,
		Source: model.EventSourceUser,
	}}
	responseJSON, _ := json.Marshal(translated)

	gen := &capturingGenerator{}
	// Override the default empty response with valid JSON.
	origGenerate := gen.Generate
	_ = origGenerate
	captureGen := &externalCapturingGenerator{response: string(responseJSON)}

	d := NewExternalDirector("ext_1", captureGen)
	d.SubmitText("look around", "")

	_, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if captureGen.lastSystem != ExternalTranslationPrompt {
		t.Errorf("system prompt mismatch")
	}
	if captureGen.lastUser != "look around" {
		t.Errorf("user prompt = %q, want %q", captureGen.lastUser, "look around")
	}
}

func TestExternalDirectorActorIDPrepended(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)
	d.Submit(ExternalInput{
		Events: []model.WorldEvent{{
			ID:       "event_1",
			Type:     model.EventTypeNote,
			ActorIDs: []model.EntityID{"char_bob"},
		}},
		ActorID: "char_alice",
	})

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if len(got[0].ActorIDs) != 2 {
		t.Fatalf("actor_ids len = %d, want 2", len(got[0].ActorIDs))
	}
	if got[0].ActorIDs[0] != "char_alice" {
		t.Errorf("actor_ids[0] = %q, want char_alice", got[0].ActorIDs[0])
	}
	if got[0].ActorIDs[1] != "char_bob" {
		t.Errorf("actor_ids[1] = %q, want char_bob", got[0].ActorIDs[1])
	}
}

func TestExternalDirectorActorIDOnTextInput(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)
	d.SubmitText("swing sword", "char_hero")

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if len(got[0].ActorIDs) != 1 || got[0].ActorIDs[0] != "char_hero" {
		t.Fatalf("actor_ids = %v, want [char_hero]", got[0].ActorIDs)
	}
}

func TestExternalDirectorActorIDNotDuplicated(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)
	d.Submit(ExternalInput{
		Events: []model.WorldEvent{{
			ID:       "event_1",
			Type:     model.EventTypeNote,
			ActorIDs: []model.EntityID{"char_alice", "char_bob"},
		}},
		ActorID: "char_alice",
	})

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if len(got[0].ActorIDs) != 2 {
		t.Fatalf("actor_ids = %v, expected no duplicate prepend", got[0].ActorIDs)
	}
}

func TestExternalDirectorConcurrentSubmit(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			d.SubmitText(fmt.Sprintf("action %d", n), "")
		}(i)
	}
	wg.Wait()

	if d.Pending() != goroutines {
		t.Fatalf("pending = %d, want %d", d.Pending(), goroutines)
	}

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if len(got) != goroutines {
		t.Fatalf("events = %d, want %d", len(got), goroutines)
	}
	if d.Pending() != 0 {
		t.Fatalf("pending after Propose = %d, want 0", d.Pending())
	}
}

func TestExternalDirectorPendingCount(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)

	if d.Pending() != 0 {
		t.Fatalf("initial pending = %d", d.Pending())
	}

	d.SubmitText("one", "")
	d.SubmitText("two", "")
	d.SubmitEvents(model.WorldEvent{ID: "e", Type: "note", Source: "x"})

	if d.Pending() != 3 {
		t.Fatalf("pending = %d, want 3", d.Pending())
	}

	_, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}

	if d.Pending() != 0 {
		t.Fatalf("pending after Propose = %d, want 0", d.Pending())
	}
}

func TestExternalDirectorMultipleInputsInOnePropose(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)

	d.SubmitEvents(
		model.WorldEvent{ID: "event_a", Type: model.EventTypeNote, Description: "A"},
	)
	d.SubmitText("narrative text", "char_narrator")
	d.Submit(ExternalInput{
		Events: []model.WorldEvent{
			{ID: "event_b", Type: model.EventTypeMove, Description: "B"},
			{ID: "event_c", Type: model.EventTypeNote, Description: "C"},
		},
		Source: "game_master",
	})

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}

	// 1 from first SubmitEvents + 1 note from SubmitText + 2 from third Submit
	if len(got) != 4 {
		t.Fatalf("expected 4 events, got %d", len(got))
	}

	if got[0].ID != "event_a" || got[0].Source != model.EventSourceUser {
		t.Errorf("event[0] = {ID:%q, Source:%q}", got[0].ID, got[0].Source)
	}

	if got[1].Type != model.EventTypeNote || got[1].ActorIDs[0] != "char_narrator" {
		t.Errorf("event[1] = {Type:%q, ActorIDs:%v}", got[1].Type, got[1].ActorIDs)
	}
	if got[1].Source != model.EventSourceUser {
		t.Errorf("event[1].Source = %q", got[1].Source)
	}

	if got[2].Source != "game_master" || got[3].Source != "game_master" {
		t.Errorf("event[2].Source=%q, event[3].Source=%q", got[2].Source, got[3].Source)
	}
}

func TestExternalDirectorTranslatorError(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", fakeGenerator{err: context.DeadlineExceeded})
	d.SubmitText("do something", "")

	_, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err == nil {
		t.Fatal("expected error from failing translator")
	}
}

func TestExternalDirectorTranslatorInvalidJSON(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", fakeGenerator{response: "not json"})
	d.SubmitText("do something", "")

	_, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err == nil {
		t.Fatal("expected parse error from invalid translator response")
	}
}

func TestExternalDirectorProposeDrainsQueue(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("ext_1", nil)
	d.SubmitText("first", "")

	got1, _ := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if len(got1) != 1 {
		t.Fatalf("first Propose: got %d events", len(got1))
	}

	got2, _ := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if len(got2) != 0 {
		t.Fatalf("second Propose should be empty, got %d events", len(got2))
	}
}

func TestExternalDirectorID(t *testing.T) {
	t.Parallel()

	d := NewExternalDirector("my_external", nil)
	if d.ID() != "my_external" {
		t.Fatalf("ID() = %q", d.ID())
	}
}

// --- test helpers ---

type externalCapturingGenerator struct {
	response   string
	lastSystem string
	lastUser   string
}

func (g *externalCapturingGenerator) Generate(_ context.Context, system, user string) (string, error) {
	g.lastSystem = system
	g.lastUser = user
	return g.response, nil
}
