package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/isnakolah/github-first-delivery/internal/github"
	"github.com/isnakolah/github-first-delivery/internal/model"
	"github.com/isnakolah/github-first-delivery/internal/writer"
	"gopkg.in/yaml.v3"
)

const version = "0.0.0-dev"
const configPath = ".github/gfd/config.yaml"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gfd:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "version":
		fmt.Println(version)
		return nil
	case "init":
		return initCommand(args[1:])
	case "doctor":
		return doctor(args[1:])
	case "context":
		return contextCommand(args[1:])
	case "validate":
		return validateCommand(args[1:])
	case "request":
		return requestCommand(args[1:])
	case "work":
		return workCommand(args[1:])
	case "evidence":
		return evidenceCommand(args[1:])
	case "journal":
		return journalCommand(args[1:])
	case "configure":
		return configureCommand(args[1:])
	case "writer":
		return writerCommand(args[1:])
	case "adopt":
		return adoptCommand(args[1:])
	case "policy", "issue", "pr":
		return fmt.Errorf("%s is reserved; implementation tracked through GitHub Project", args[0])
	default:
		return usage()
	}
}

func adoptCommand(args []string) error {
	fs := flag.NewFlagSet("adopt", flag.ContinueOnError)
	owner := fs.String("owner", "", "GitHub owner")
	repo := fs.String("repo", "", "repository")
	apply := applyFlag(fs)
	asJSON := jsonFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *owner == "" || *repo == "" {
		return errors.New("--owner and --repo are required")
	}
	output, err := exec.Command("gh", "api", "repos/"+*owner+"/"+*repo+"/issues?state=all&per_page=100").Output()
	if err != nil {
		return fmt.Errorf("audit repository: %w", err)
	}
	var issues []json.RawMessage
	if err := json.Unmarshal(output, &issues); err != nil {
		return err
	}
	report := map[string]any{"owner": *owner, "repository": *repo, "issues_found": len(issues), "safe_to_adopt": len(issues) == 0, "note": "adopt never reclassifies existing Issues"}
	if !*apply {
		return printValue(report, *asJSON)
	}
	if len(issues) != 0 {
		return errors.New("adopt refuses repository with existing Issues; audit and migrate explicitly")
	}
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("%s already exists", configPath)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	c := model.Config{SchemaVersion: model.ConfigVersion, Owner: *owner, Repository: *repo, WikiMode: "off", DefaultBranch: "main", Areas: []string{"stable"}, WriterVersion: version, Policy: model.Policy{LeaseTTLMinutes: 120, ReconcileMins: 5}, Project: model.Project{Fields: map[string]string{}}}
	if err := c.Validate(); err != nil {
		return err
	}
	raw, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, raw, 0644); err != nil {
		return err
	}
	return printValue(report, *asJSON)
}
func usage() error {
	return errors.New("usage: gfd {init|doctor|context|validate|request|work|evidence|journal|version}")
}
func applyFlag(fs *flag.FlagSet) *bool {
	return fs.Bool("apply", false, "perform state-changing action")
}
func jsonFlag(fs *flag.FlagSet) *bool { return fs.Bool("json", false, "JSON output") }

func initCommand(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	owner := fs.String("owner", "", "GitHub owner")
	repo := fs.String("repo", "", "repository")
	project := fs.String("project-name", "", "Project title")
	wiki := fs.String("wiki", "off", "off|journal")
	branch := fs.String("default-branch", "main", "default branch")
	areas := fs.String("areas", "stable", "comma-separated areas")
	visibility := fs.String("visibility", "public", "public|private")
	apply := applyFlag(fs)
	yes := fs.Bool("yes", false, "confirm bootstrap")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*apply || !*yes {
		return errors.New("init requires --apply --yes")
	}
	if *owner == "" || *repo == "" || *project == "" {
		return errors.New("--owner, --repo, and --project-name are required")
	}
	if *visibility != "public" && *visibility != "private" {
		return errors.New("visibility must be public or private")
	}
	c := model.Config{SchemaVersion: model.ConfigVersion, Owner: *owner, Repository: *repo, WikiMode: *wiki, DefaultBranch: *branch, Areas: split(*areas), WriterVersion: version, Policy: model.Policy{LeaseTTLMinutes: 120, ReconcileMins: 5}, Project: model.Project{Title: *project, Fields: map[string]string{}}}
	if err := c.Validate(); err != nil {
		return err
	}
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("%s already exists; use adopt for existing repositories", configPath)
	}
	projectInfo, err := provisionGitHub(*owner, *repo, *visibility, *project)
	if err != nil {
		return err
	}
	if err := provisionProjectContract(*owner, *repo, projectInfo.Number); err != nil {
		return err
	}
	c.Project.ID = projectInfo.ID
	c.Project.Number = projectInfo.Number
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	raw, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, raw, 0644); err != nil {
		return err
	}
	fmt.Printf("provisioned %s/%s and Project #%d; wrote %s\n", *owner, *repo, projectInfo.Number, configPath)
	return nil
}

type createdProject struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
}

func provisionGitHub(owner, repo, visibility, projectName string) (createdProject, error) {
	remote := owner + "/" + repo
	if err := exec.Command("gh", "repo", "view", remote).Run(); err != nil {
		flag := "--public"
		if visibility == "private" {
			flag = "--private"
		}
		if output, createErr := exec.Command("gh", "repo", "create", remote, flag).CombinedOutput(); createErr != nil {
			return createdProject{}, fmt.Errorf("create repository %s: %s", remote, strings.TrimSpace(string(output)))
		}
	}
	output, err := exec.Command("gh", "project", "create", "--owner", owner, "--title", projectName, "--format", "json").Output()
	if err != nil {
		return createdProject{}, fmt.Errorf("create Project: %w", err)
	}
	var project createdProject
	if err := json.Unmarshal(output, &project); err != nil {
		return createdProject{}, err
	}
	if project.ID == "" || project.Number == 0 {
		return createdProject{}, errors.New("GitHub returned incomplete Project identity")
	}
	return project, nil
}

func provisionProjectContract(owner, repo string, projectNumber int) error {
	labels := []string{"kind:epic", "kind:contract", "kind:story", "kind:task", "kind:defect", "kind:gate", "kind:decision"}
	for _, label := range labels {
		if output, err := exec.Command("gh", "label", "create", label, "--color", "1D76DB", "--force", "--repo", owner+"/"+repo).CombinedOutput(); err != nil {
			return fmt.Errorf("create label %s: %s", label, strings.TrimSpace(string(output)))
		}
	}
	fields := []struct{ name, kind, options string }{
		{"Kind", "SINGLE_SELECT", "Epic,Contract,Story,Task,Defect,Gate,Decision"},
		{"Area", "SINGLE_SELECT", "Stable"},
		{"Priority", "SINGLE_SELECT", "P0,P1,P2,P3"},
		{"Proof", "SINGLE_SELECT", "Not started,Local verified,CI verified,Target environment pending,External/provider pending,Complete"},
		{"Lease holder", "TEXT", ""}, {"Lease expires", "TEXT", ""}, {"Branch", "TEXT", ""}, {"State fingerprint", "TEXT", ""},
	}
	for _, field := range fields {
		args := []string{"project", "field-create", fmt.Sprint(projectNumber), "--owner", owner, "--name", field.name, "--data-type", field.kind}
		if field.options != "" {
			args = append(args, "--single-select-options", field.options)
		}
		if output, err := exec.Command("gh", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("create Project field %s: %s", field.name, strings.TrimSpace(string(output)))
		}
	}
	return nil
}
func split(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
func loadConfig() (model.Config, error) {
	var c model.Config
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return c, err
	}
	err = yaml.Unmarshal(raw, &c)
	if err != nil {
		return c, err
	}
	return c, c.Validate()
}
func printValue(v any, asJSON bool) error {
	if asJSON {
		raw, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	fmt.Printf("%+v\n", v)
	return nil
}
func doctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	asJSON := jsonFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := loadConfig()
	report := map[string]any{"gfd_version": version, "config": err == nil, "github_token": os.Getenv("GITHUB_TOKEN") != "", "repository": ""}
	if err == nil {
		report["repository"] = c.Owner + "/" + c.Repository
	}
	return printValue(report, *asJSON)
}
func contextCommand(args []string) error {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	asJSON := jsonFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	return printValue(c, *asJSON)
}
func validateCommand(args []string) error {
	_, err := loadConfig()
	if err != nil {
		return err
	}
	fmt.Println("valid: repository configuration")
	return nil
}

func requestCommand(args []string) error {
	if len(args) == 0 || args[0] != "status" {
		return errors.New("usage: gfd request status --issue-number N --issue-id ID --action ACTION --actor ACTOR --fingerprint SHA --apply")
	}
	return submitRequest("status", args[1:], nil)
}
func workCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: gfd work {claim|start|renew|release|block|resume}")
	}
	return submitRequest(args[0], args[1:], nil)
}
func evidenceCommand(args []string) error {
	if len(args) == 0 || args[0] != "submit" {
		return errors.New("usage: gfd evidence submit")
	}
	e := &writer.Evidence{}
	return submitRequest("evidence.submit", args[1:], e)
}
func submitRequest(action string, args []string, evidence *writer.Evidence) error {
	fs := flag.NewFlagSet(action, flag.ContinueOnError)
	number := fs.Int("issue-number", 0, "issue number")
	id := fs.String("issue-id", "", "issue node ID")
	actor := fs.String("actor", os.Getenv("USER"), "actor identity")
	fingerprint := fs.String("fingerprint", "", "observed fingerprint")
	apply := applyFlag(fs)
	lease := fs.String("lease-expires", "", "RFC3339 expiry")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*apply {
		return errors.New("changing commands require --apply")
	}
	if *number < 1 || *id == "" || *fingerprint == "" {
		return errors.New("--issue-number, --issue-id, and --fingerprint are required")
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	r := writer.Request{ID: fmt.Sprintf("%d", time.Now().UnixNano()), Action: action, IssueID: *id, Actor: *actor, ExpectedFingerprint: *fingerprint, LeaseExpiresAt: *lease, Evidence: evidence}
	body, err := writer.RenderRequest(r)
	if err != nil {
		return err
	}
	comment, err := github.NewClient().CreateComment(context.Background(), c.Owner, c.Repository, *number, body)
	if err != nil {
		return err
	}
	fmt.Printf("request %s posted as comment %d\n", r.ID, comment.ID)
	return nil
}
func journalCommand(args []string) error {
	if len(args) == 0 || args[0] != "render" {
		return errors.New("usage: gfd journal render")
	}
	fmt.Println("journal render delegated to serialized writer")
	return nil
}

func configureCommand(args []string) error {
	if len(args) == 0 || args[0] != "writer-token" {
		return errors.New("usage: gfd configure writer-token --apply < token")
	}
	fs := flag.NewFlagSet("configure writer-token", flag.ContinueOnError)
	apply := applyFlag(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if !*apply {
		return errors.New("writer-token configuration requires --apply")
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	cmd := exec.Command("gh", "secret", "set", "GFD_WRITER_TOKEN", "--repo", c.Owner+"/"+c.Repository)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set GFD_WRITER_TOKEN: %w", err)
	}
	fmt.Println("GFD_WRITER_TOKEN configured; token was not read, printed, or persisted locally")
	return nil
}

func writerCommand(args []string) error {
	if len(args) == 0 || args[0] != "run" {
		return errors.New("usage: gfd writer run --issue-number N [--apply]")
	}
	fs := flag.NewFlagSet("writer run", flag.ContinueOnError)
	number := fs.Int("issue-number", 0, "issue number")
	apply := applyFlag(fs)
	asJSON := jsonFlag(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *number < 1 {
		return errors.New("--issue-number is required")
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	client := github.NewClient()
	comments, err := client.ListComments(context.Background(), c.Owner, c.Repository, *number)
	if err != nil {
		return err
	}
	pending, rejected := writer.Pending(comments)
	receiptsApplied := 0
	if *apply {
		for _, item := range pending {
			rejected = append(rejected, writer.AcceptanceReceipt(item.Request))
		}
		for _, receipt := range rejected {
			body, err := writer.RejectionComment(receipt)
			if err != nil {
				return err
			}
			if _, err := client.CreateComment(context.Background(), c.Owner, c.Repository, *number, body); err != nil {
				return err
			}
			receiptsApplied++
		}
	}
	ids := make([]string, 0, len(pending))
	for _, item := range pending {
		ids = append(ids, item.Request.ID)
	}
	return printValue(map[string]any{"issue_number": *number, "pending_request_ids": ids, "receipts_applied": receiptsApplied, "apply": *apply, "note": "receipt acceptance records request; lifecycle state mutation remains later Writer work"}, *asJSON)
}
