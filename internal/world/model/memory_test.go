package model

import (
	"reflect"
	"testing"
)

func validMemory() MemoryRecord {
	return MemoryRecord{
		ID:      "mem-1",
		Owner:   MemoryOwner{Kind: MemoryOwnerKindCharacter, ID: "char-1"},
		Content: "saw the king fall",
	}
}

func TestMemoryValidateAcceptsEmotion(t *testing.T) {
	m := validMemory()
	m.Emotion = map[string]float64{"fear": 0.8, "anger": -0.3}
	if err := m.Validate(); err != nil {
		t.Fatalf("valid emotion: %v", err)
	}
}

func TestMemoryValidateRejectsEmotionOutOfRange(t *testing.T) {
	m := validMemory()
	m.Emotion = map[string]float64{"joy": 1.5}
	if err := m.Validate(); err == nil {
		t.Fatal("should reject emotion > 1")
	}

	m.Emotion = map[string]float64{"fear": -1.5}
	if err := m.Validate(); err == nil {
		t.Fatal("should reject emotion < -1")
	}
}

func TestMemoryValidateRejectsEmptyEmotionKey(t *testing.T) {
	m := validMemory()
	m.Emotion = map[string]float64{"": 0.5}
	if err := m.Validate(); err == nil {
		t.Fatal("should reject empty emotion key")
	}
}

func TestMemoryValidateAcceptsSource(t *testing.T) {
	sources := []string{
		MemorySourceDirectExperience, MemorySourceHearsay, MemorySourceDeduction,
		MemorySourceSystemExtraction, MemorySourceAuthorSeed, MemorySourceScript,
	}
	for _, src := range sources {
		m := validMemory()
		m.Source = src
		if err := m.Validate(); err != nil {
			t.Errorf("valid source %q: %v", src, err)
		}
	}
}

func TestMemoryValidateRejectsUnknownSource(t *testing.T) {
	m := validMemory()
	m.Source = "telepathy"
	if err := m.Validate(); err == nil {
		t.Fatal("should reject unknown source")
	}
}

func TestMemoryValidateAcceptsDecay(t *testing.T) {
	modes := []string{
		MemoryDecayNone, MemoryDecayFadeConfidence, MemoryDecayFadeImportance,
		MemoryDecaySummarizeAfter, MemoryDecayArchiveAfter,
	}
	for _, mode := range modes {
		m := validMemory()
		m.Decay = &MemoryDecay{Mode: mode}
		if err := m.Validate(); err != nil {
			t.Errorf("valid decay mode %q: %v", mode, err)
		}
	}
}

func TestMemoryValidateDecayWithHalfLife(t *testing.T) {
	m := validMemory()
	m.Decay = &MemoryDecay{Mode: MemoryDecayFadeConfidence, HalfLife: "7d", Preserve: true}
	if err := m.Validate(); err != nil {
		t.Fatalf("valid decay with half_life: %v", err)
	}
}

func TestMemoryValidateRejectsUnknownDecayMode(t *testing.T) {
	m := validMemory()
	m.Decay = &MemoryDecay{Mode: "vanish"}
	if err := m.Validate(); err == nil {
		t.Fatal("should reject unknown decay mode")
	}
}

func TestMemoryValidateRejectsDecayWithoutMode(t *testing.T) {
	m := validMemory()
	m.Decay = &MemoryDecay{}
	if err := m.Validate(); err == nil {
		t.Fatal("should reject decay without mode")
	}
}

func TestMemoryValidateAcceptsZeroValueNewFields(t *testing.T) {
	m := validMemory()
	if err := m.Validate(); err != nil {
		t.Fatalf("should accept zero-value new fields: %v", err)
	}
}

func TestMemoryValidateAcceptsZeroTimestamps(t *testing.T) {
	m := validMemory()
	if err := m.Validate(); err != nil {
		t.Fatalf("zero-value timestamps should be valid: %v", err)
	}
	if !reflect.DeepEqual(m.CreatedAt, WorldTime{}) {
		t.Fatal("expected zero CreatedAt")
	}
	if !reflect.DeepEqual(m.UpdatedAt, WorldTime{}) {
		t.Fatal("expected zero UpdatedAt")
	}
	if !reflect.DeepEqual(m.LastAccess, WorldTime{}) {
		t.Fatal("expected zero LastAccess")
	}
}

func TestMemoryValidateAcceptsPopulatedTimestamps(t *testing.T) {
	m := validMemory()
	m.CreatedAt = WorldTime{Kind: WorldTimeTick, Tick: 5}
	m.UpdatedAt = WorldTime{Kind: WorldTimeTick, Tick: 10}
	m.LastAccess = WorldTime{Kind: WorldTimeTick, Tick: 12}
	if err := m.Validate(); err != nil {
		t.Fatalf("populated timestamps should be valid: %v", err)
	}
}
