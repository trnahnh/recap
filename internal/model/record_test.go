package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func strptr(s string) *string { return &s }

func f32ptr(f float32) *float32 { return &f }

func fullRecord() Record {
	ts := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	return Record{
		ID:             "11111111-1111-1111-1111-111111111111",
		ProjectID:      "22222222-2222-2222-2222-222222222222",
		SessionID:      strptr("33333333-3333-3333-3333-333333333333"),
		RecordType:     RecordTypeDecision,
		Title:          "Use PostgreSQL",
		Task:           "Pick a storage backend",
		Summary:        "Chose Postgres for concurrent writes",
		ChosenApproach: strptr("PostgreSQL via Docker"),
		Rationale:      strptr("row-level locking"),
		Status:         RecordStatusActive,
		Confidence:     f32ptr(0.9),
		CreatedBy:      "claude-code",
		CreatedAt:      ts,
		UpdatedAt:      ts,
		Alternatives: []Alternative{
			{
				ID:       "44444444-4444-4444-4444-444444444444",
				RecordID: "11111111-1111-1111-1111-111111111111",
				Approach: "SQLite",
				Result:   strptr("rejected"),
				Reason:   strptr("file-level locking"),
				Position: 0,
			},
		},
		Files: []RecordFile{
			{
				ID:         "55555555-5555-5555-5555-555555555555",
				RecordID:   "11111111-1111-1111-1111-111111111111",
				FilePath:   "docs/ARCHITECTURE_DECISIONS.md",
				CommitHash: strptr("abc123"),
			},
		},
		Relationships: []Relationship{
			{
				ID:               "66666666-6666-6666-6666-666666666666",
				RecordID:         "11111111-1111-1111-1111-111111111111",
				TargetRecordID:   "77777777-7777-7777-7777-777777777777",
				RelationshipType: RelationshipSupersedes,
				CreatedAt:        ts,
			},
		},
	}
}

func TestRecordJSONRoundTrip(t *testing.T) {
	in := fullRecord()
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out Record
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\n in = %+v\nout = %+v", in, out)
	}
}

func TestRecordJSONDocumentedShape(t *testing.T) {
	data, err := json.Marshal(fullRecord())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)
	for _, key := range []string{`"record_type":"decision"`, `"status":"active"`, `"relationship_type":"supersedes"`, `"project_id"`, `"chosen_approach"`, `"target_record_id"`} {
		if !strings.Contains(s, key) {
			t.Errorf("marshaled JSON missing %s\ngot: %s", key, s)
		}
	}
}

func TestNewRecordEmitsEmptyArrays(t *testing.T) {
	data, err := json.Marshal(NewRecord())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)
	for _, key := range []string{`"alternatives":[]`, `"files":[]`, `"relationships":[]`} {
		if !strings.Contains(s, key) {
			t.Errorf("NewRecord JSON missing %s\ngot: %s", key, s)
		}
	}
}

func TestRecordValidate(t *testing.T) {
	ok := fullRecord()
	if err := ok.Validate(); err != nil {
		t.Errorf("valid record rejected: %v", err)
	}

	noID := fullRecord()
	noID.ID = ""
	if err := noID.Validate(); err != nil {
		t.Errorf("DB-assigned empty id should pass: %v", err)
	}

	badID := fullRecord()
	badID.ID = "not-a-uuid"
	if err := badID.Validate(); err == nil {
		t.Error("malformed id should fail")
	}

	noProject := fullRecord()
	noProject.ProjectID = ""
	if err := noProject.Validate(); err == nil {
		t.Error("missing project_id should fail")
	}

	badConfidence := fullRecord()
	badConfidence.Confidence = f32ptr(1.5)
	if err := badConfidence.Validate(); err == nil {
		t.Error("confidence > 1 should fail")
	}

	badEnum := fullRecord()
	badEnum.RecordType = RecordType("nope")
	if err := badEnum.Validate(); err == nil {
		t.Error("invalid record_type should fail")
	}

	emptyTitle := fullRecord()
	emptyTitle.Title = "   "
	if err := emptyTitle.Validate(); err == nil {
		t.Error("blank title should fail")
	}
}
