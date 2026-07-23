package model

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RootPath  string    `json:"root_path"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Record struct {
	ID             string         `json:"id"`
	ProjectID      string         `json:"project_id"`
	SessionID      *string        `json:"session_id,omitempty"`
	RecordType     RecordType     `json:"record_type"`
	Title          string         `json:"title"`
	Task           string         `json:"task"`
	Summary        string         `json:"summary"`
	ChosenApproach *string        `json:"chosen_approach,omitempty"`
	Rationale      *string        `json:"rationale,omitempty"`
	Status         RecordStatus   `json:"status"`
	Confidence     *float32       `json:"confidence,omitempty"`
	CreatedBy      string         `json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Alternatives   []Alternative  `json:"alternatives"`
	Files          []RecordFile   `json:"files"`
	Relationships  []Relationship `json:"relationships"`
}

type Alternative struct {
	ID       string  `json:"id"`
	RecordID string  `json:"record_id"`
	Approach string  `json:"approach"`
	Result   *string `json:"result,omitempty"`
	Reason   *string `json:"reason,omitempty"`
	Position int     `json:"position"`
}

type RecordFile struct {
	ID         string  `json:"id"`
	RecordID   string  `json:"record_id"`
	FilePath   string  `json:"file_path"`
	CommitHash *string `json:"commit_hash,omitempty"`
}

type Relationship struct {
	ID               string           `json:"id"`
	RecordID         string           `json:"record_id"`
	TargetRecordID   string           `json:"target_record_id"`
	RelationshipType RelationshipType `json:"relationship_type"`
	CreatedAt        time.Time        `json:"created_at"`
}

func NewRecord() Record {
	return Record{
		Status:        RecordStatusDraft,
		Alternatives:  []Alternative{},
		Files:         []RecordFile{},
		Relationships: []Relationship{},
	}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validUUID(s string) bool {
	return uuidPattern.MatchString(s)
}

func (r *Record) Validate() error {
	if r.ID != "" && !validUUID(r.ID) {
		return fmt.Errorf("model: record id %q is not a valid uuid", r.ID)
	}
	if !validUUID(r.ProjectID) {
		return fmt.Errorf("model: project_id %q is not a valid uuid", r.ProjectID)
	}
	if r.SessionID != nil && !validUUID(*r.SessionID) {
		return fmt.Errorf("model: session_id %q is not a valid uuid", *r.SessionID)
	}
	if !r.RecordType.Valid() {
		return fmt.Errorf("model: invalid record_type %q", r.RecordType)
	}
	if !r.Status.Valid() {
		return fmt.Errorf("model: invalid record_status %q", r.Status)
	}
	if strings.TrimSpace(r.Title) == "" {
		return fmt.Errorf("model: title is required")
	}
	if strings.TrimSpace(r.Task) == "" {
		return fmt.Errorf("model: task is required")
	}
	if strings.TrimSpace(r.Summary) == "" {
		return fmt.Errorf("model: summary is required")
	}
	if strings.TrimSpace(r.CreatedBy) == "" {
		return fmt.Errorf("model: created_by is required")
	}
	if r.Confidence != nil && (*r.Confidence < 0 || *r.Confidence > 1) {
		return fmt.Errorf("model: confidence %v out of range [0,1]", *r.Confidence)
	}
	for i := range r.Alternatives {
		if err := r.Alternatives[i].validate(); err != nil {
			return fmt.Errorf("model: alternative %d: %w", i, err)
		}
	}
	for i := range r.Files {
		if err := r.Files[i].validate(); err != nil {
			return fmt.Errorf("model: file %d: %w", i, err)
		}
	}
	for i := range r.Relationships {
		if err := r.Relationships[i].validate(); err != nil {
			return fmt.Errorf("model: relationship %d: %w", i, err)
		}
	}
	return nil
}

func (a *Alternative) validate() error {
	if a.ID != "" && !validUUID(a.ID) {
		return fmt.Errorf("id %q is not a valid uuid", a.ID)
	}
	if a.RecordID != "" && !validUUID(a.RecordID) {
		return fmt.Errorf("record_id %q is not a valid uuid", a.RecordID)
	}
	if strings.TrimSpace(a.Approach) == "" {
		return fmt.Errorf("approach is required")
	}
	return nil
}

func (f *RecordFile) validate() error {
	if f.ID != "" && !validUUID(f.ID) {
		return fmt.Errorf("id %q is not a valid uuid", f.ID)
	}
	if f.RecordID != "" && !validUUID(f.RecordID) {
		return fmt.Errorf("record_id %q is not a valid uuid", f.RecordID)
	}
	if strings.TrimSpace(f.FilePath) == "" {
		return fmt.Errorf("file_path is required")
	}
	return nil
}

func (rel *Relationship) validate() error {
	if rel.ID != "" && !validUUID(rel.ID) {
		return fmt.Errorf("id %q is not a valid uuid", rel.ID)
	}
	if rel.RecordID != "" && !validUUID(rel.RecordID) {
		return fmt.Errorf("record_id %q is not a valid uuid", rel.RecordID)
	}
	if !validUUID(rel.TargetRecordID) {
		return fmt.Errorf("target_record_id %q is not a valid uuid", rel.TargetRecordID)
	}
	if !rel.RelationshipType.Valid() {
		return fmt.Errorf("invalid relationship_type %q", rel.RelationshipType)
	}
	return nil
}
