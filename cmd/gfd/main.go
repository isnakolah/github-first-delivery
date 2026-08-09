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
	case "issue":
		return issueCommand(args[1:])
	case "policy", "pr":
		return fmt.Errorf("%s is reserved; implementation tracked through GitHub Project", args[0])
	default:
		return usage()
	}
}

func issueCommand(args []string) error {
	if len(args) > 0 && args[0] == "link-parent" {
		return linkParentCommand(args[1:])
	}
	if len(args) > 0 && args[0] == "add-blocker" {
		return addBlockerCommand(args[1:])
	}
	if len(args) == 0 || args[0] != "create" {
		return errors.New("usage: gfd issue {create|link-parent}")
	}
	fs := flag.NewFlagSet("issue create", flag.ContinueOnError)
	title := fs.String("title", "", "Issue title")
	kind := fs.String("kind", "", "epic|contract|story|task|defect|gate|decision")
	bodyFile := fs.String("body-file", "", "contract Markdown file")
	apply := applyFlag(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if !*apply {
		return errors.New("issue creation requires --apply")
	}
	if *title == "" || *bodyFile == "" {
		return errors.New("--title and --body-file are required")
	}
	allowed := map[string]bool{"epic": true, "contract": true, "story": true, "task": true, "defect": true, "gate": true, "decision": true}
	if !allowed[*kind] {
		return errors.New("invalid --kind")
	}
	body, err := os.ReadFile(*bodyFile)
	if err != nil {
		return err
	}
	if *kind != "epic" && model.ValidateWorkContract(string(body)) != nil {
		return errors.New("work Issue body does not satisfy work:v1 contract")
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	cmd := exec.Command("gh", "issue", "create", "--repo", c.Owner+"/"+c.Repository, "--title", *title, "--label", "kind:"+*kind, "--body-file", *bodyFile)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func addBlockerCommand(args []string) error {
	fs := flag.NewFlagSet("issue add-blocker", flag.ContinueOnError)
	issue := fs.Int("issue", 0, "blocked Issue number")
	blocker := fs.Int("blocker", 0, "blocking Issue number")
	apply := applyFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*apply {
		return errors.New("blocker link requires --apply")
	}
	if *issue < 1 || *blocker < 1 || *issue == *blocker {
		return errors.New("distinct --issue and --blocker are required")
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	query := fmt.Sprintf("query { repository(owner:\"%s\",name:\"%s\") { i:issue(number:%d){id} b:issue(number:%d){id} } }", c.Owner, c.Repository, *issue, *blocker)
	output, err := exec.Command("gh", "api", "graphql", "-f", "query="+query).Output()
	if err != nil {
		return err
	}
	var response struct {
		Data struct {
			Repository struct {
				I struct {
					ID string `json:"id"`
				} `json:"i"`
				B struct {
					ID string `json:"id"`
				} `json:"b"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return err
	}
	if response.Data.Repository.I.ID == "" || response.Data.Repository.B.ID == "" {
		return errors.New("Issue or blocker not found")
	}
	mutation := fmt.Sprintf("mutation { addBlockedBy(input:{issueId:\"%s\",blockingIssueId:\"%s\"}) { issue { number } } }", response.Data.Repository.I.ID, response.Data.Repository.B.ID)
	cmd := exec.Command("gh", "api", "graphql", "-f", "query="+mutation)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func linkParentCommand(args []string) error {
	fs := flag.NewFlagSet("issue link-parent", flag.ContinueOnError)
	parent := fs.Int("parent", 0, "parent Issue number")
	child := fs.Int("child", 0, "child Issue number")
	apply := applyFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*apply {
		return errors.New("parent link requires --apply")
	}
	if *parent < 1 || *child < 1 || *parent == *child {
		return errors.New("distinct --parent and --child are required")
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	query := fmt.Sprintf("query { repository(owner:\"%s\",name:\"%s\") { p:issue(number:%d){id} c:issue(number:%d){id} } }", c.Owner, c.Repository, *parent, *child)
	output, err := exec.Command("gh", "api", "graphql", "-f", "query="+query).Output()
	if err != nil {
		return err
	}
	var response struct {
		Data struct {
			Repository struct {
				P struct {
					ID string `json:"id"`
				} `json:"p"`
				C struct {
					ID string `json:"id"`
				} `json:"c"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return err
	}
	if response.Data.Repository.P.ID == "" || response.Data.Repository.C.ID == "" {
		return errors.New("parent or child Issue not found")
	}
	mutation := fmt.Sprintf("mutation { addSubIssue(input:{issueId:\"%s\",subIssueId:\"%s\"}) { subIssue { number } } }", response.Data.Repository.P.ID, response.Data.Repository.C.ID)
	cmd := exec.Command("gh", "api", "graphql", "-f", "query="+mutation)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
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
	number := fs.Int("issue-number", 0, "Issue number for live state fingerprint")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	if *number > 0 {
		state, err := loadLiveWork(c, *number)
		if err != nil {
			return err
		}
		fingerprint, err := fingerprintLiveWork(state)
		if err != nil {
			return err
		}
		return printValue(map[string]any{"config": c, "issue_number": *number, "issue_id": state.IssueID, "status": state.Status, "lease_holder": state.Holder, "lease_expires": state.Expiry, "branch": state.Branch, "state_fingerprint": fingerprint}, *asJSON)
	}
	return printValue(c, *asJSON)
}
func validateCommand(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	asJSON := jsonFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`query { repository(owner:%q,name:%q) { issues(first:100,states:[OPEN,CLOSED]) { nodes {
id number body state parent { id }
blockedBy(first:100) { nodes { id } }
labels(first:100) { nodes { name } }
projectItems(first:20) { nodes { fieldValues(first:30) { nodes {
... on ProjectV2ItemFieldSingleSelectValue { name field { ... on ProjectV2SingleSelectField { name } } }
... on ProjectV2ItemFieldTextValue { text field { ... on ProjectV2Field { name } } }
} } } }
} } } }`, c.Owner, c.Repository)
	output, err := exec.Command("gh", "api", "graphql", "-f", "query="+query).Output()
	if err != nil {
		return err
	}
	var response struct {
		Data struct {
			Repository struct {
				Issues struct {
					Nodes []struct {
						ID     string `json:"id"`
						Number int    `json:"number"`
						Body   string `json:"body"`
						State  string `json:"state"`
						Parent *struct {
							ID string `json:"id"`
						} `json:"parent"`
						BlockedBy struct {
							Nodes []struct {
								ID string `json:"id"`
							} `json:"nodes"`
						} `json:"blockedBy"`
						Labels struct {
							Nodes []struct {
								Name string `json:"name"`
							} `json:"nodes"`
						} `json:"labels"`
						ProjectItems struct {
							Nodes []struct {
								FieldValues struct {
									Nodes []struct {
										Name  string `json:"name"`
										Text  string `json:"text"`
										Field struct {
											Name string `json:"name"`
										} `json:"field"`
									} `json:"nodes"`
								} `json:"fieldValues"`
							} `json:"nodes"`
						} `json:"projectItems"`
					} `json:"nodes"`
				} `json:"issues"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return err
	}
	issues := make([]model.Issue, 0, len(response.Data.Repository.Issues.Nodes))
	for _, node := range response.Data.Repository.Issues.Nodes {
		kind, area := "", ""
		for _, label := range node.Labels.Nodes {
			if strings.HasPrefix(label.Name, "kind:") {
				kind = strings.TrimPrefix(label.Name, "kind:")
				kind = strings.ToUpper(kind[:1]) + kind[1:]
			}
			if strings.HasPrefix(label.Name, "area:") {
				area = strings.TrimPrefix(label.Name, "area:")
			}
		}
		status, projectKind, projectArea, branch := "", "", "", ""
		for _, item := range node.ProjectItems.Nodes {
			for _, value := range item.FieldValues.Nodes {
				switch value.Field.Name {
				case "Status":
					status = value.Name
				case "Kind":
					projectKind = value.Name
				case "Area":
					projectArea = value.Name
				case "Branch":
					branch = value.Text
				}
			}
		}
		parent := ""
		if node.Parent != nil {
			parent = node.Parent.ID
		}
		blockers := make([]string, 0, len(node.BlockedBy.Nodes))
		for _, blocker := range node.BlockedBy.Nodes {
			blockers = append(blockers, blocker.ID)
		}
		issues = append(issues, model.Issue{ID: node.ID, Number: node.Number, Kind: kind, Status: status, State: node.State, Area: area, ProjectKind: projectKind, ProjectArea: projectArea, Branch: branch, ParentID: parent, BlockerIDs: blockers, Body: node.Body})
	}
	err = model.ValidateGraph(issues)
	openIssues := 0
	for _, issue := range issues {
		if issue.State != "CLOSED" {
			openIssues++
		}
	}
	report := map[string]any{"repository": c.Owner + "/" + c.Repository, "open_issues": openIssues, "valid": err == nil}
	if *asJSON {
		if err != nil {
			report["error"] = err.Error()
		}
		return printValue(report, true)
	}
	if err != nil {
		return err
	}
	fmt.Printf("valid: %d open Issues\n", openIssues)
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
	branch := fs.String("branch", "", "NNN/short-description branch")
	status := fs.String("status", "", "requested Project Status")
	pr := fs.String("pr", "", "pull request URL")
	var finalSHA, ciURL, commands, environments, criteria, artifacts, documentation, risks, boundary *string
	if evidence != nil {
		finalSHA = fs.String("final-sha", "", "final merged commit SHA")
		ciURL = fs.String("ci-url", "", "CI run URL")
		commands = fs.String("commands", "", "exact verification commands")
		environments = fs.String("environments", "", "verified environments")
		criteria = fs.String("criteria", "", "acceptance-criteria result")
		artifacts = fs.String("artifacts", "", "artifact, screenshot, or log URLs; None: reason")
		documentation = fs.String("documentation", "", "documentation changes; None: reason")
		risks = fs.String("risks", "", "known residual risks; None")
		boundary = fs.String("boundary", "", "local|CI|target host|provider|staging|production|release")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*apply {
		return errors.New("changing commands require --apply")
	}
	if *number < 1 || *id == "" || *fingerprint == "" {
		return errors.New("--issue-number, --issue-id, and --fingerprint are required")
	}
	if evidence != nil {
		*evidence = writer.Evidence{FinalSHA: *finalSHA, CIURL: *ciURL, Commands: *commands, Environments: *environments, Criteria: *criteria, Artifacts: *artifacts, Documentation: *documentation, Risks: *risks, Boundary: *boundary}
		if err := evidence.Validate(); err != nil {
			return err
		}
		if *pr == "" {
			return errors.New("evidence submit requires --pr")
		}
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	r := writer.Request{ID: fmt.Sprintf("%d", time.Now().UnixNano()), Action: action, IssueID: *id, Actor: *actor, ExpectedFingerprint: *fingerprint, LeaseExpiresAt: *lease, Branch: *branch, Status: *status, PR: *pr, Evidence: evidence}
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
	if len(args) == 0 {
		return errors.New("usage: gfd writer {run|reconcile}")
	}
	if args[0] == "reconcile" {
		return writerReconcileCommand(args[1:])
	}
	if args[0] != "run" {
		return errors.New("usage: gfd writer {run|reconcile}")
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
			receipt := applyWriterRequest(c, *number, item.Request)
			rejected = append(rejected, receipt)
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
	return printValue(map[string]any{"issue_number": *number, "pending_request_ids": ids, "receipts_applied": receiptsApplied, "apply": *apply, "note": "lifecycle requests require fresh fingerprint and configured Project membership"}, *asJSON)
}

func writerReconcileCommand(args []string) error {
	fs := flag.NewFlagSet("writer reconcile", flag.ContinueOnError)
	apply := applyFlag(fs)
	asJSON := jsonFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	output, err := exec.Command("gh", "issue", "list", "--repo", c.Owner+"/"+c.Repository, "--state", "open", "--limit", "100", "--json", "number").Output()
	if err != nil {
		return err
	}
	var issues []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(output, &issues); err != nil {
		return err
	}
	report := map[string]any{"issues_scanned": len(issues), "requests_receipted": 0, "leases_reclaimed": 0, "apply": *apply}
	for _, issue := range issues {
		receipted, reclaimed, err := reconcileIssue(c, issue.Number, *apply)
		if err != nil {
			return fmt.Errorf("reconcile Issue #%d: %w", issue.Number, err)
		}
		report["requests_receipted"] = report["requests_receipted"].(int) + receipted
		report["leases_reclaimed"] = report["leases_reclaimed"].(int) + reclaimed
	}
	return printValue(report, *asJSON)
}

func reconcileIssue(c model.Config, number int, apply bool) (int, int, error) {
	client := github.NewClient()
	comments, err := client.ListComments(context.Background(), c.Owner, c.Repository, number)
	if err != nil {
		return 0, 0, err
	}
	pending, rejected := writer.Pending(comments)
	receipted := 0
	if apply {
		for _, item := range pending {
			rejected = append(rejected, applyWriterRequest(c, number, item.Request))
		}
		for _, receipt := range rejected {
			body, err := writer.RejectionComment(receipt)
			if err != nil {
				return 0, 0, err
			}
			if _, err := client.CreateComment(context.Background(), c.Owner, c.Repository, number, body); err != nil {
				return 0, 0, err
			}
			receipted++
		}
	}
	state, err := loadLiveWork(c, number)
	if err != nil {
		return receipted, 0, err
	}
	expires, _ := time.Parse(time.RFC3339, state.Expiry)
	next, reclaimed := writer.ReclaimExpired(writer.WorkState{Status: state.Status, Lease: writer.Lease{Holder: state.Holder, Expires: expires, Branch: state.Branch}}, time.Now())
	if !reclaimed {
		return receipted, 0, nil
	}
	if !apply {
		return receipted, 1, nil
	}
	if err := updateLiveWork(c, state.ItemID, next); err != nil {
		return receipted, 0, err
	}
	requestID := fmt.Sprintf("lease-expiry-%s-%s", state.IssueID, state.Expiry)
	for _, comment := range comments {
		if strings.Contains(comment.Body, "request="+requestID) {
			return receipted, 1, nil
		}
	}
	body, err := writer.RenderReceipt(writer.Receipt{RequestID: requestID, Result: "accepted", Detail: "expired lease cleared; Issue returned to Ready", At: time.Now().UTC()})
	if err != nil {
		return receipted, 0, err
	}
	if _, err := client.CreateComment(context.Background(), c.Owner, c.Repository, number, body); err != nil {
		return receipted, 0, err
	}
	return receipted + 1, 1, nil
}

type liveWork struct {
	IssueID    string
	IssueState string
	UpdatedAt  string
	ParentID   string
	Blockers   []liveBlocker
	ItemID     string
	Status     string
	Holder     string
	Expiry     string
	Branch     string
}

type liveBlocker struct{ ID, State string }

func applyWriterRequest(c model.Config, number int, request writer.Request) writer.Receipt {
	state, err := loadLiveWork(c, number)
	if err != nil {
		return rejectedReceipt(request, err)
	}
	if state.IssueID != request.IssueID {
		return rejectedReceipt(request, errors.New("request Issue ID does not match target Issue"))
	}
	actual, err := writer.RequireFreshFingerprint(request.ExpectedFingerprint, liveWorkFingerprint(state))
	if err != nil {
		return writer.Receipt{RequestID: request.ID, Fingerprint: actual, Result: "rejected", Detail: err.Error(), At: time.Now().UTC()}
	}
	if request.Action == "claim" {
		for _, blocker := range state.Blockers {
			if blocker.State != "CLOSED" {
				return rejectedReceipt(request, fmt.Errorf("unresolved blocker %s prevents claim", blocker.ID))
			}
		}
	}
	current := writer.WorkState{Status: state.Status, Lease: writer.Lease{Holder: state.Holder, Branch: state.Branch}}
	if state.Expiry != "" {
		current.Lease.Expires, _ = time.Parse(time.RFC3339, state.Expiry)
	}
	next, err := writer.ApplyLifecycle(current, request, time.Now())
	if err != nil {
		return rejectedReceipt(request, err)
	}
	if err := updateLiveWork(c, state.ItemID, next); err != nil {
		return rejectedReceipt(request, err)
	}
	receipt := writer.Receipt{RequestID: request.ID, Fingerprint: actual, Result: "accepted", Detail: "lifecycle state changed to " + next.Status, At: time.Now().UTC()}
	if request.Action == "evidence.submit" {
		receipt.Detail = "evidence recorded; lifecycle state changed to Evidence pending"
		receipt.Evidence = request.Evidence
	}
	return receipt
}

func fingerprintLiveWork(state liveWork) (string, error) {
	return writer.StateFingerprint(liveWorkFingerprint(state))
}

func liveWorkFingerprint(state liveWork) writer.Fingerprint {
	fingerprint := writer.Fingerprint{IssueID: state.IssueID, IssueState: state.IssueState, UpdatedAt: state.UpdatedAt, ParentID: state.ParentID, Project: map[string]string{"status": state.Status, "lease_holder": state.Holder, "lease_expires": state.Expiry, "branch": state.Branch}}
	for _, blocker := range state.Blockers {
		fingerprint.DependencyIDs = append(fingerprint.DependencyIDs, blocker.ID)
	}
	return fingerprint
}

func rejectedReceipt(request writer.Request, err error) writer.Receipt {
	return writer.Receipt{RequestID: request.ID, Fingerprint: request.ExpectedFingerprint, Result: "rejected", Detail: err.Error(), At: time.Now().UTC()}
}

func loadLiveWork(c model.Config, number int) (liveWork, error) {
	query := fmt.Sprintf(`query { repository(owner:%q,name:%q) { issue(number:%d) {
id state updatedAt parent { id } blockedBy(first:100) { nodes { id state } }
projectItems(first:20) { nodes { id project { id } fieldValues(first:30) { nodes {
... on ProjectV2ItemFieldSingleSelectValue { name field { ... on ProjectV2SingleSelectField { name } } }
... on ProjectV2ItemFieldTextValue { text field { ... on ProjectV2Field { name } } }
} } } }
} } }`, c.Owner, c.Repository, number)
	output, err := exec.Command("gh", "api", "graphql", "-f", "query="+query).Output()
	if err != nil {
		return liveWork{}, err
	}
	var response struct {
		Data struct {
			Repository struct {
				Issue struct {
					ID        string `json:"id"`
					State     string `json:"state"`
					UpdatedAt string `json:"updatedAt"`
					Parent    *struct {
						ID string `json:"id"`
					} `json:"parent"`
					BlockedBy struct {
						Nodes []struct {
							ID    string `json:"id"`
							State string `json:"state"`
						} `json:"nodes"`
					} `json:"blockedBy"`
					ProjectItems struct {
						Nodes []struct {
							ID      string `json:"id"`
							Project struct {
								ID string `json:"id"`
							} `json:"project"`
							FieldValues struct {
								Nodes []struct {
									Name  string `json:"name"`
									Text  string `json:"text"`
									Field struct {
										Name string `json:"name"`
									} `json:"field"`
								} `json:"nodes"`
							} `json:"fieldValues"`
						} `json:"nodes"`
					} `json:"projectItems"`
				} `json:"issue"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return liveWork{}, err
	}
	issue := response.Data.Repository.Issue
	if issue.ID == "" {
		return liveWork{}, errors.New("Issue not found")
	}
	state := liveWork{IssueID: issue.ID, IssueState: issue.State, UpdatedAt: issue.UpdatedAt}
	if issue.Parent != nil {
		state.ParentID = issue.Parent.ID
	}
	for _, blocker := range issue.BlockedBy.Nodes {
		state.Blockers = append(state.Blockers, liveBlocker{ID: blocker.ID, State: blocker.State})
	}
	for _, item := range issue.ProjectItems.Nodes {
		if item.Project.ID != c.Project.ID {
			continue
		}
		state.ItemID = item.ID
		for _, value := range item.FieldValues.Nodes {
			switch value.Field.Name {
			case "Status":
				state.Status = value.Name
			case "Lease holder":
				state.Holder = value.Text
			case "Lease expires":
				state.Expiry = value.Text
			case "Branch":
				state.Branch = value.Text
			}
		}
	}
	if state.ItemID == "" {
		return liveWork{}, errors.New("Issue is not a member of configured Project")
	}
	return state, nil
}

type projectField struct {
	ID      string
	Options map[string]string
}

func updateLiveWork(c model.Config, itemID string, state writer.WorkState) error {
	fields, err := configuredProjectFields(c)
	if err != nil {
		return err
	}
	values := []struct {
		name  string
		value string
		text  bool
	}{
		{"Lease holder", state.Lease.Holder, true},
		{"Lease expires", leaseText(state.Lease.Expires), true},
		{"Branch", state.Lease.Branch, true},
		{"Status", state.Status, false},
	}
	for _, value := range values {
		field, ok := fields[value.name]
		if !ok {
			return fmt.Errorf("configured Project lacks field %q", value.name)
		}
		args := []string{"project", "item-edit", "--id", itemID, "--project-id", c.Project.ID, "--field-id", field.ID}
		if value.text {
			if value.value == "" {
				args = append(args, "--clear")
			} else {
				args = append(args, "--text", value.value)
			}
		} else {
			option, ok := field.Options[value.value]
			if !ok {
				return fmt.Errorf("Project field %s has no option %q", value.name, value.value)
			}
			args = append(args, "--single-select-option-id", option)
		}
		if output, err := exec.Command("gh", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("update %s: %s", value.name, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func leaseText(expiry time.Time) string {
	if expiry.IsZero() {
		return ""
	}
	return expiry.UTC().Format(time.RFC3339)
}

func configuredProjectFields(c model.Config) (map[string]projectField, error) {
	output, err := exec.Command("gh", "project", "field-list", fmt.Sprint(c.Project.Number), "--owner", c.Owner, "--format", "json").Output()
	if err != nil {
		return nil, err
	}
	var response struct {
		Fields []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Options []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"options"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, err
	}
	fields := make(map[string]projectField, len(response.Fields))
	for _, raw := range response.Fields {
		field := projectField{ID: raw.ID, Options: map[string]string{}}
		for _, option := range raw.Options {
			field.Options[option.Name] = option.ID
		}
		fields[raw.Name] = field
	}
	return fields, nil
}
