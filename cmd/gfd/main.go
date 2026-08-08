package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
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
	case "writer":
		return writerCommand(args[1:])
	case "policy", "issue", "pr", "adopt":
		return fmt.Errorf("%s is reserved; implementation tracked through GitHub Project", args[0])
	default:
		return usage()
	}
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
	fmt.Printf("wrote %s; provision GitHub Project through gfd writer bootstrap\n", configPath)
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
