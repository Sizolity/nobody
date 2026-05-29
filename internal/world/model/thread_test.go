package model

import "testing"

func validThread() WorldThread {
	return WorldThread{
		ID:     "thread_1",
		Kind:   ThreadKindMystery,
		Title:  "Find the killer",
		Status: ThreadStatusOpen,
	}
}

func TestThreadWithGoalsValidates(t *testing.T) {
	thread := validThread()
	thread.Goals = []ThreadGoal{
		{
			ID:          "goal_1",
			OwnerID:     "char_alice",
			Description: "Discover the murderer",
			DesiredState: []Condition{{
				Kind: ConditionKindFact,
				Path: "facts.murder.solved",
			}},
		},
		{
			ID:          "goal_2",
			Description: "Survive the investigation",
			Optional:    true,
		},
	}
	if err := thread.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestThreadRejectsGoalWithoutDescription(t *testing.T) {
	thread := validThread()
	thread.Goals = []ThreadGoal{{ID: "goal_1"}}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for goal without description")
	}
}

func TestThreadRejectsGoalWithInvalidCondition(t *testing.T) {
	thread := validThread()
	thread.Goals = []ThreadGoal{{
		ID:          "goal_1",
		Description: "A goal",
		DesiredState: []Condition{{
			Kind: "bad_kind",
			Path: "some.path",
		}},
	}}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for goal with invalid condition kind")
	}
}

func TestThreadWithInvalidStakeSeverityRejected(t *testing.T) {
	thread := validThread()
	thread.Stakes = []ThreadStake{{
		Description: "Alice might die",
		EntityIDs:   []EntityID{"char_alice"},
		Severity:    1.5,
	}}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for stake severity > 1")
	}

	thread.Stakes[0].Severity = -0.1
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for negative stake severity")
	}
}

func TestThreadRejectsStakeWithoutDescription(t *testing.T) {
	thread := validThread()
	thread.Stakes = []ThreadStake{{Severity: 0.5}}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for stake without description")
	}
}

func TestThreadWithInvalidClueReliabilityRejected(t *testing.T) {
	thread := validThread()
	thread.Clues = []ThreadClue{{
		ID:          "clue_1",
		Content:     "A bloody knife was found",
		Reliability: 1.2,
	}}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for clue reliability > 1")
	}

	thread.Clues[0].Reliability = -0.5
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for negative clue reliability")
	}
}

func TestThreadRejectsClueWithoutContent(t *testing.T) {
	thread := validThread()
	thread.Clues = []ThreadClue{{ID: "clue_1", Reliability: 0.5}}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for clue without content")
	}
}

func TestThreadWithInvalidBranchWeightRejected(t *testing.T) {
	thread := validThread()
	thread.Branches = []ThreadBranch{{
		ResultHint: "Alice confronts the suspect",
		Weight:     2.0,
	}}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for branch weight > 1")
	}

	thread.Branches[0].Weight = -1.0
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for negative branch weight")
	}
}

func TestThreadWithBranchInvalidConditionRejected(t *testing.T) {
	thread := validThread()
	thread.Branches = []ThreadBranch{{
		TriggerCondition: []Condition{{Kind: ConditionKindState}},
		Weight:           0.5,
	}}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for branch with condition missing path")
	}
}

func TestThreadWithDeadlineAcceptsWorldTime(t *testing.T) {
	thread := validThread()
	thread.Deadline = &WorldTime{Kind: WorldTimeTick, Tick: 100}
	if err := thread.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	thread.Deadline = &WorldTime{Kind: WorldTimeDay, Label: "Day 3"}
	if err := thread.Validate(); err != nil {
		t.Fatalf("Validate returned error for label deadline: %v", err)
	}
}

func TestThreadZeroValueSubfieldsValidate(t *testing.T) {
	thread := validThread()
	if err := thread.Validate(); err != nil {
		t.Fatalf("Validate returned error for zero-value subfields: %v", err)
	}

	if thread.Goals != nil || thread.Stakes != nil || thread.Clues != nil || thread.Branches != nil || thread.Deadline != nil {
		t.Fatal("zero-value thread should have nil subfields")
	}
}

func TestThreadWithAllSubstructuresValidates(t *testing.T) {
	thread := validThread()
	thread.Goals = []ThreadGoal{{
		ID:          "goal_1",
		Description: "Solve the mystery",
		DesiredState: []Condition{{
			Kind:     ConditionKindFact,
			Path:     "facts.case.solved",
			Operator: "eq",
			Value:    Value{Kind: ValueKindBoolean, Raw: true},
		}},
	}}
	thread.Stakes = []ThreadStake{{
		Description: "Someone may be wrongfully accused",
		Severity:    0.8,
	}}
	thread.Clues = []ThreadClue{{
		ID:           "clue_1",
		Content:      "Footprints near the window",
		KnownBy:      []EntityID{"char_alice"},
		Reliability:  0.7,
		PointsTo:     []EntityID{"char_bob"},
		DiscoveredAt: "event_5",
	}}
	thread.Branches = []ThreadBranch{{
		TriggerCondition: []Condition{{
			Kind: ConditionKindState,
			Path: "entities.char_bob.caught",
		}},
		ResultHint: "Bob confesses",
		Weight:     0.6,
	}}
	thread.Deadline = &WorldTime{Kind: WorldTimeTick, Tick: 50}

	if err := thread.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestConditionValidateRequiresKindAndPath(t *testing.T) {
	c := Condition{Path: "some.path"}
	if err := c.Validate(); err == nil {
		t.Fatal("Validate returned nil without kind")
	}

	c = Condition{Kind: ConditionKindState}
	if err := c.Validate(); err == nil {
		t.Fatal("Validate returned nil without path")
	}
}

func TestConditionValidateRejectsUnsupportedKind(t *testing.T) {
	c := Condition{Kind: "unsupported", Path: "some.path"}
	if err := c.Validate(); err == nil {
		t.Fatal("Validate returned nil for unsupported kind")
	}
}

func TestConditionValidateAcceptsAllKinds(t *testing.T) {
	for _, kind := range []string{
		ConditionKindState, ConditionKindFact, ConditionKindRelation,
		ConditionKindMemory, ConditionKindStat,
	} {
		c := Condition{Kind: kind, Path: "some.path"}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate returned error for kind %q: %v", kind, err)
		}
	}
}
