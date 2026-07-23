package model

import "testing"

func TestParseRecordType(t *testing.T) {
	valid := []string{"decision", "failed_attempt", "constraint", "discovery", "open_question"}
	for _, s := range valid {
		got, err := ParseRecordType(s)
		if err != nil {
			t.Errorf("ParseRecordType(%q) unexpected error: %v", s, err)
		}
		if got.String() != s {
			t.Errorf("ParseRecordType(%q).String() = %q", s, got.String())
		}
		if !got.Valid() {
			t.Errorf("ParseRecordType(%q) produced invalid value", s)
		}
	}
	if _, err := ParseRecordType("Decision"); err == nil {
		t.Error("ParseRecordType(\"Decision\") expected error, got nil")
	}
}

func TestParseRecordStatus(t *testing.T) {
	valid := []string{"draft", "active", "superseded", "archived", "invalid"}
	for _, s := range valid {
		got, err := ParseRecordStatus(s)
		if err != nil {
			t.Errorf("ParseRecordStatus(%q) unexpected error: %v", s, err)
		}
		if got.String() != s {
			t.Errorf("ParseRecordStatus(%q).String() = %q", s, got.String())
		}
		if !got.Valid() {
			t.Errorf("ParseRecordStatus(%q) produced invalid value", s)
		}
	}
	if _, err := ParseRecordStatus("pending"); err == nil {
		t.Error("ParseRecordStatus(\"pending\") expected error, got nil")
	}
}

func TestParseRelationshipType(t *testing.T) {
	valid := []string{"supersedes", "relates_to", "contradicts", "depends_on", "duplicates"}
	for _, s := range valid {
		got, err := ParseRelationshipType(s)
		if err != nil {
			t.Errorf("ParseRelationshipType(%q) unexpected error: %v", s, err)
		}
		if got.String() != s {
			t.Errorf("ParseRelationshipType(%q).String() = %q", s, got.String())
		}
		if !got.Valid() {
			t.Errorf("ParseRelationshipType(%q) produced invalid value", s)
		}
	}
	if _, err := ParseRelationshipType("supersede"); err == nil {
		t.Error("ParseRelationshipType(\"supersede\") expected error, got nil")
	}
}
