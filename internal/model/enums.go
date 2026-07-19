package model

import "fmt"

type RecordType string

const (
	RecordTypeDecision      RecordType = "decision"
	RecordTypeFailedAttempt RecordType = "failed_attempt"
	RecordTypeConstraint    RecordType = "constraint"
	RecordTypeDiscovery     RecordType = "discovery"
	RecordTypeOpenQuestion  RecordType = "open_question"
)

func (t RecordType) String() string { return string(t) }

func (t RecordType) Valid() bool {
	switch t {
	case RecordTypeDecision, RecordTypeFailedAttempt, RecordTypeConstraint, RecordTypeDiscovery, RecordTypeOpenQuestion:
		return true
	default:
		return false
	}
}

func ParseRecordType(s string) (RecordType, error) {
	t := RecordType(s)
	if !t.Valid() {
		return "", fmt.Errorf("model: invalid record_type %q", s)
	}
	return t, nil
}

type RecordStatus string

const (
	RecordStatusDraft      RecordStatus = "draft"
	RecordStatusActive     RecordStatus = "active"
	RecordStatusSuperseded RecordStatus = "superseded"
	RecordStatusArchived   RecordStatus = "archived"
	RecordStatusInvalid    RecordStatus = "invalid"
)

func (s RecordStatus) String() string { return string(s) }

func (s RecordStatus) Valid() bool {
	switch s {
	case RecordStatusDraft, RecordStatusActive, RecordStatusSuperseded, RecordStatusArchived, RecordStatusInvalid:
		return true
	default:
		return false
	}
}

func ParseRecordStatus(s string) (RecordStatus, error) {
	rs := RecordStatus(s)
	if !rs.Valid() {
		return "", fmt.Errorf("model: invalid record_status %q", s)
	}
	return rs, nil
}

type RelationshipType string

const (
	RelationshipSupersedes  RelationshipType = "supersedes"
	RelationshipRelatesTo   RelationshipType = "relates_to"
	RelationshipContradicts RelationshipType = "contradicts"
	RelationshipDependsOn   RelationshipType = "depends_on"
	RelationshipDuplicates  RelationshipType = "duplicates"
)

func (t RelationshipType) String() string { return string(t) }

func (t RelationshipType) Valid() bool {
	switch t {
	case RelationshipSupersedes, RelationshipRelatesTo, RelationshipContradicts, RelationshipDependsOn, RelationshipDuplicates:
		return true
	default:
		return false
	}
}

func ParseRelationshipType(s string) (RelationshipType, error) {
	t := RelationshipType(s)
	if !t.Valid() {
		return "", fmt.Errorf("model: invalid relationship_type %q", s)
	}
	return t, nil
}
