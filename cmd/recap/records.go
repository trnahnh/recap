package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/trnahnh/recap/internal/model"
	"github.com/trnahnh/recap/internal/store"
)

type recordScope struct {
	fs          *flag.FlagSet
	configPath  *string
	projectPath *string
}

func newRecordScope(name string) recordScope {
	fs := flag.NewFlagSet("recap "+name, flag.ContinueOnError)
	return recordScope{
		fs:          fs,
		configPath:  fs.String("config", "", "path to config file (default: OS config dir)"),
		projectPath: fs.String("project", "", "project root path (default: current directory)"),
	}
}

func (s recordScope) open(ctx context.Context) (*app, model.Project, error) {
	configPath, err := resolveConfigPath(*s.configPath)
	if err != nil {
		return nil, model.Project{}, err
	}
	a, err := openApp(ctx, configPath)
	if err != nil {
		return nil, model.Project{}, err
	}
	project, err := resolveProject(ctx, a.store, *s.projectPath)
	if err != nil {
		a.close()
		return nil, model.Project{}, err
	}
	return a, project, nil
}

func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func recordIDArg(positional []string, usage string) (string, error) {
	if len(positional) < 1 {
		return "", fmt.Errorf("usage: %s", usage)
	}
	id := positional[0]
	if !model.IsUUID(id) {
		return "", fmt.Errorf("invalid record id %q (expected a uuid)", id)
	}
	return id, nil
}

func describeRecordErr(err error, id string, project model.Project) error {
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("record %s not found in project %s", id, project.Name)
	}
	return err
}

type listFilter struct {
	statuses []model.RecordStatus
	types    []model.RecordType
}

func parseListFilter(status, recordType string) (listFilter, error) {
	var f listFilter
	for _, raw := range splitCSV(status) {
		s, err := model.ParseRecordStatus(raw)
		if err != nil {
			return listFilter{}, fmt.Errorf("--status: %w", err)
		}
		f.statuses = append(f.statuses, s)
	}
	for _, raw := range splitCSV(recordType) {
		t, err := model.ParseRecordType(raw)
		if err != nil {
			return listFilter{}, fmt.Errorf("--type: %w", err)
		}
		f.types = append(f.types, t)
	}
	return f, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runList(ctx context.Context, args []string, w io.Writer) error {
	scope := newRecordScope("list")
	status := scope.fs.String("status", "", "comma-separated statuses to include (default: all)")
	recordType := scope.fs.String("type", "", "comma-separated record types to include (default: all)")
	if err := scope.fs.Parse(args); err != nil {
		return err
	}
	filter, err := parseListFilter(*status, *recordType)
	if err != nil {
		return err
	}

	a, project, err := scope.open(ctx)
	if err != nil {
		return err
	}
	defer a.close()

	records, err := a.store.ListRecords(ctx, project.ID, store.RecordFilter{
		Statuses:    filter.statuses,
		RecordTypes: filter.types,
	})
	if err != nil {
		return err
	}
	return writeRecordTable(w, records)
}

func writeRecordTable(w io.Writer, records []model.Record) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tTYPE\tUPDATED\tTITLE")
	for _, r := range records {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			r.ID, strings.ToUpper(string(r.Status)), r.RecordType,
			r.UpdatedAt.Local().Format(time.DateTime), r.Title)
	}
	return tw.Flush()
}

func runShow(ctx context.Context, args []string, w io.Writer) error {
	scope := newRecordScope("show")
	positional, err := parseInterspersed(scope.fs, args)
	if err != nil {
		return err
	}
	id, err := recordIDArg(positional, "recap show <id> [--project <path>]")
	if err != nil {
		return err
	}

	a, project, err := scope.open(ctx)
	if err != nil {
		return err
	}
	defer a.close()

	record, err := a.store.GetRecord(ctx, project.ID, id)
	if err != nil {
		return describeRecordErr(err, id, project)
	}
	writeRecord(w, record)
	return nil
}

func writeRecord(w io.Writer, r model.Record) {
	if r.Status == model.RecordStatusDraft {
		fmt.Fprintf(w, "[DRAFT] %s\n", r.Title)
		fmt.Fprintln(w, "This record is a draft and is not shown to AI tools until approved (recap approve <id>).")
	} else {
		fmt.Fprintln(w, r.Title)
	}
	fmt.Fprintf(w, "id:         %s\n", r.ID)
	fmt.Fprintf(w, "status:     %s\n", strings.ToUpper(string(r.Status)))
	fmt.Fprintf(w, "type:       %s\n", r.RecordType)
	fmt.Fprintf(w, "created by: %s\n", r.CreatedBy)
	fmt.Fprintf(w, "created:    %s\n", r.CreatedAt.Local().Format(time.RFC3339))
	fmt.Fprintf(w, "updated:    %s\n", r.UpdatedAt.Local().Format(time.RFC3339))
	if r.Confidence != nil {
		fmt.Fprintf(w, "confidence: %.2f\n", *r.Confidence)
	}
	fmt.Fprintf(w, "\ntask:\n  %s\n", r.Task)
	fmt.Fprintf(w, "\nsummary:\n  %s\n", r.Summary)
	if r.ChosenApproach != nil {
		fmt.Fprintf(w, "\nchosen approach:\n  %s\n", *r.ChosenApproach)
	}
	if r.Rationale != nil {
		fmt.Fprintf(w, "\nrationale:\n  %s\n", *r.Rationale)
	}
	if len(r.Alternatives) > 0 {
		fmt.Fprintln(w, "\nalternatives:")
		for _, alt := range r.Alternatives {
			line := "  - " + alt.Approach
			if alt.Result != nil {
				line += " [" + *alt.Result + "]"
			}
			if alt.Reason != nil {
				line += ": " + *alt.Reason
			}
			fmt.Fprintln(w, line)
		}
	}
	if len(r.Files) > 0 {
		fmt.Fprintln(w, "\nfiles:")
		for _, f := range r.Files {
			line := "  - " + f.FilePath
			if f.CommitHash != nil {
				line += " @ " + *f.CommitHash
			}
			fmt.Fprintln(w, line)
		}
	}
	if len(r.Relationships) > 0 {
		fmt.Fprintln(w, "\nrelationships:")
		for _, rel := range r.Relationships {
			fmt.Fprintf(w, "  - %s %s\n", rel.RelationshipType, rel.TargetRecordID)
		}
	}
	if len(r.IncomingRelationships) > 0 {
		fmt.Fprintln(w, "\nreferenced by:")
		for _, rel := range r.IncomingRelationships {
			fmt.Fprintf(w, "  - %s %s\n", rel.RecordID, rel.RelationshipType)
		}
	}
}

type editableFields struct {
	Title          string   `json:"title"`
	Task           string   `json:"task"`
	Summary        string   `json:"summary"`
	ChosenApproach *string  `json:"chosen_approach"`
	Rationale      *string  `json:"rationale"`
	Confidence     *float32 `json:"confidence"`
}

func editableFrom(r model.Record) editableFields {
	return editableFields{
		Title:          r.Title,
		Task:           r.Task,
		Summary:        r.Summary,
		ChosenApproach: r.ChosenApproach,
		Rationale:      r.Rationale,
		Confidence:     r.Confidence,
	}
}

func (e editableFields) applyTo(r *model.Record) {
	r.Title = e.Title
	r.Task = e.Task
	r.Summary = e.Summary
	r.ChosenApproach = e.ChosenApproach
	r.Rationale = e.Rationale
	r.Confidence = e.Confidence
}

type editFlags struct {
	title, task, summary, approach, rationale string
	confidence                                float64
	set                                       map[string]bool
}

func (f *editFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.title, "title", "", "new title")
	fs.StringVar(&f.task, "task", "", "new task")
	fs.StringVar(&f.summary, "summary", "", "new summary")
	fs.StringVar(&f.approach, "approach", "", "new chosen approach")
	fs.StringVar(&f.rationale, "rationale", "", "new rationale")
	fs.Float64Var(&f.confidence, "confidence", 0, "new confidence in [0,1]")
}

func (f *editFlags) collect(fs *flag.FlagSet) {
	f.set = map[string]bool{}
	fs.Visit(func(fl *flag.Flag) { f.set[fl.Name] = true })
}

func (f *editFlags) anyField() bool {
	for _, name := range []string{"title", "task", "summary", "approach", "rationale", "confidence"} {
		if f.set[name] {
			return true
		}
	}
	return false
}

func (f *editFlags) applyTo(e *editableFields) error {
	if f.set["title"] {
		e.Title = f.title
	}
	if f.set["task"] {
		e.Task = f.task
	}
	if f.set["summary"] {
		e.Summary = f.summary
	}
	if f.set["approach"] {
		e.ChosenApproach = optionalString(f.approach)
	}
	if f.set["rationale"] {
		e.Rationale = optionalString(f.rationale)
	}
	if f.set["confidence"] {
		if f.confidence < 0 || f.confidence > 1 {
			return fmt.Errorf("--confidence must be in [0,1], got %v", f.confidence)
		}
		c := float32(f.confidence)
		e.Confidence = &c
	}
	return nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func runEdit(ctx context.Context, args []string, w io.Writer) error {
	scope := newRecordScope("edit")
	var flags editFlags
	flags.register(scope.fs)
	positional, err := parseInterspersed(scope.fs, args)
	if err != nil {
		return err
	}
	flags.collect(scope.fs)
	id, err := recordIDArg(positional, "recap edit <id> [--title ...] [--task ...] [--summary ...] [--approach ...] [--rationale ...] [--confidence ...]")
	if err != nil {
		return err
	}

	a, project, err := scope.open(ctx)
	if err != nil {
		return err
	}
	defer a.close()

	record, err := a.store.GetRecord(ctx, project.ID, id)
	if err != nil {
		return describeRecordErr(err, id, project)
	}

	fields := editableFrom(record)
	if flags.anyField() {
		if err := flags.applyTo(&fields); err != nil {
			return err
		}
	} else {
		edited, changed, err := editInEditor(fields)
		if err != nil {
			return err
		}
		if !changed {
			fmt.Fprintln(w, "no changes.")
			return nil
		}
		fields = edited
	}
	fields.applyTo(&record)

	updated, err := a.store.UpdateRecord(ctx, record)
	if err != nil {
		return describeRecordErr(err, id, project)
	}
	fmt.Fprintf(w, "updated %s\n", updated.ID)
	return nil
}

func editInEditor(fields editableFields) (editableFields, bool, error) {
	original, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return editableFields{}, false, fmt.Errorf("encoding record: %w", err)
	}
	tmp, err := os.CreateTemp("", "recap-edit-*.json")
	if err != nil {
		return editableFields{}, false, fmt.Errorf("creating edit buffer: %w", err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.Write(original); err != nil {
		tmp.Close()
		return editableFields{}, false, fmt.Errorf("writing edit buffer: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return editableFields{}, false, fmt.Errorf("closing edit buffer: %w", err)
	}

	name, editorArgs := editorCommand()
	cmd := exec.Command(name, append(editorArgs, path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return editableFields{}, false, fmt.Errorf("running editor %s: %w", name, err)
	}

	edited, err := os.ReadFile(path)
	if err != nil {
		return editableFields{}, false, fmt.Errorf("reading edit buffer: %w", err)
	}
	if strings.TrimSpace(string(edited)) == strings.TrimSpace(string(original)) {
		return fields, false, nil
	}
	var out editableFields
	if err := json.Unmarshal(edited, &out); err != nil {
		return editableFields{}, false, fmt.Errorf("parsing edited record: %w", err)
	}
	return out, true, nil
}

func editorCommand() (string, []string) {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			parts := strings.Fields(v)
			return parts[0], parts[1:]
		}
	}
	if runtime.GOOS == "windows" {
		return "notepad", nil
	}
	return "vi", nil
}

func runDelete(ctx context.Context, args []string, w io.Writer) error {
	scope := newRecordScope("delete")
	positional, err := parseInterspersed(scope.fs, args)
	if err != nil {
		return err
	}
	id, err := recordIDArg(positional, "recap delete <id> [--project <path>]")
	if err != nil {
		return err
	}

	a, project, err := scope.open(ctx)
	if err != nil {
		return err
	}
	defer a.close()

	if err := a.store.DeleteRecord(ctx, project.ID, id); err != nil {
		return describeRecordErr(err, id, project)
	}
	fmt.Fprintf(w, "deleted %s\n", id)
	return nil
}

func runArchive(ctx context.Context, args []string, w io.Writer) error {
	return runTransition(ctx, args, w, "archive", (*store.Store).Archive)
}

func runApprove(ctx context.Context, args []string, w io.Writer) error {
	return runTransition(ctx, args, w, "approve", (*store.Store).Approve)
}

func runTransition(
	ctx context.Context, args []string, w io.Writer, name string,
	apply func(*store.Store, context.Context, string, string) (model.Record, error),
) error {
	scope := newRecordScope(name)
	positional, err := parseInterspersed(scope.fs, args)
	if err != nil {
		return err
	}
	id, err := recordIDArg(positional, "recap "+name+" <id> [--project <path>]")
	if err != nil {
		return err
	}

	a, project, err := scope.open(ctx)
	if err != nil {
		return err
	}
	defer a.close()

	record, err := apply(a.store, ctx, project.ID, id)
	if err != nil {
		return describeRecordErr(err, id, project)
	}
	fmt.Fprintf(w, "%s is now %s\n", record.ID, strings.ToUpper(string(record.Status)))
	return nil
}
