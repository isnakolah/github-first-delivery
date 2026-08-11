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

	"github.com/isnakolah/github-first-delivery/internal/bootstrap"
	"github.com/isnakolah/github-first-delivery/internal/github"
	"github.com/isnakolah/github-first-delivery/internal/journal"
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
	case "pr":
		return prCommand(args[1:])
	case "policy":
		return fmt.Errorf("policy is reserved; implementation tracked through GitHub Project")
	default:
		return usage()
	}
}

func prCommand(args []string) error {
	if len(args) == 0 || args[0] != "link" {
		return errors.New("usage: gfd pr link --issue-number N --issue-id ID --fingerprint SHA --pr URL --apply")
	}
	return submitRequest("pr.link", args[1:], nil)
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
	output, err := ghOutput("api", "graphql", "-f", "query="+query)
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
	fieldIDs, err := provisionProjectContract(*owner, *repo, projectInfo.Number, c.Areas)
	if err != nil {
		return err
	}
	c.Project.ID = projectInfo.ID
	c.Project.Number = projectInfo.Number
	c.Project.Fields = fieldIDs
	if err := bootstrap.Install(".", *owner, projectInfo.Number); err != nil {
		return err
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

func provisionProjectContract(owner, repo string, projectNumber int, areas []string) (map[string]string, error) {
	labels := []string{"kind:epic", "kind:contract", "kind:story", "kind:task", "kind:defect", "kind:gate", "kind:decision"}
	for _, area := range areas {
		labels = append(labels, "area:"+area)
	}
	for _, label := range labels {
		if output, err := exec.Command("gh", "label", "create", label, "--color", "1D76DB", "--force", "--repo", owner+"/"+repo).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("create label %s: %s", label, strings.TrimSpace(string(output)))
		}
	}
	areaOptions := make([]string, 0, len(areas))
	for _, area := range areas {
		areaOptions = append(areaOptions, strings.ToUpper(area[:1])+area[1:])
	}
	fields := []struct{ name, kind, options string }{
		{"Kind", "SINGLE_SELECT", "Epic,Contract,Story,Task,Defect,Gate,Decision"},
		{"Area", "SINGLE_SELECT", strings.Join(areaOptions, ",")},
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
			return nil, fmt.Errorf("create Project field %s: %s", field.name, strings.TrimSpace(string(output)))
		}
	}
	projectFields, err := readProjectFields(owner, projectNumber)
	if err != nil {
		return nil, err
	}
	status, ok := projectFields["Status"]
	if !ok {
		return nil, errors.New("Project default Status field not found")
	}
	if err := setStatusOptions(status.ID); err != nil {
		return nil, err
	}
	ids := map[string]string{}
	for key, name := range map[string]string{"status": "Status", "kind": "Kind", "area": "Area", "priority": "Priority", "proof": "Proof", "lease_holder": "Lease holder", "lease_expires": "Lease expires", "branch": "Branch", "state_fingerprint": "State fingerprint"} {
		field, ok := projectFields[name]
		if !ok {
			return nil, fmt.Errorf("Project field %q missing after provisioning", name)
		}
		ids[key] = field.ID
	}
	return ids, nil
}

func setStatusOptions(fieldID string) error {
	options := []struct{ name, color string }{
		{"Backlog", "GRAY"}, {"Ready", "BLUE"}, {"Claimed", "YELLOW"}, {"In progress", "ORANGE"}, {"In review", "PURPLE"}, {"Evidence pending", "PINK"}, {"Blocked", "RED"}, {"Done", "GREEN"}, {"Cancelled", "GRAY"}, {"Archived", "GRAY"},
	}
	parts := make([]string, 0, len(options))
	for _, option := range options {
		parts = append(parts, fmt.Sprintf(`{name:%q,color:%s,description:""}`, option.name, option.color))
	}
	mutation := fmt.Sprintf("mutation { updateProjectV2Field(input:{fieldId:%q,singleSelectOptions:[%s]}) { projectV2Field { ... on ProjectV2SingleSelectField { id } } } }", fieldID, strings.Join(parts, ","))
	if output, err := exec.Command("gh", "api", "graphql", "-f", "query="+mutation).CombinedOutput(); err != nil {
		return fmt.Errorf("configure Project Status options: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func readProjectFields(owner string, projectNumber int) (map[string]projectField, error) {
	output, err := exec.Command("gh", "project", "field-list", fmt.Sprint(projectNumber), "--owner", owner, "--format", "json").Output()
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
	report := map[string]any{"gfd_version": version, "config": err == nil, "github_token": github.NewClient().Token != "", "repository": ""}
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
		return errors.New("usage: gfd journal render --request-id ID --date YYYY-MM-DD --issue '#N' --pr URL --outcome TEXT --proof TEXT --boundary TEXT --next-blocker TEXT")
	}
	fs := flag.NewFlagSet("journal render", flag.ContinueOnError)
	requestID := fs.String("request-id", "", "Writer request ID")
	date := fs.String("date", "", "UTC date, YYYY-MM-DD")
	issue := fs.String("issue", "", "Issue reference")
	pr := fs.String("pr", "", "pull request URL or None")
	outcome := fs.String("outcome", "", "outcome summary")
	proof := fs.String("proof", "", "proof summary")
	boundary := fs.String("boundary", "", "proof boundary")
	nextBlocker := fs.String("next-blocker", "", "next blocker or None")
	asJSON := jsonFlag(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *requestID == "" || *date == "" || *issue == "" || *outcome == "" || *proof == "" || *boundary == "" || *nextBlocker == "" {
		return errors.New("journal render requires request-id, date, issue, outcome, proof, boundary, and next-blocker")
	}
	entry := journal.Entry{RequestID: *requestID, Date: *date, Issue: *issue, PR: *pr, Outcome: *outcome, Proof: *proof, Boundary: *boundary, NextBlocker: *nextBlocker}
	if *asJSON {
		return printValue(map[string]any{"entry": entry, "markdown": journal.Render(entry)}, true)
	}
	fmt.Print(journal.Render(entry))
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
			receipt := applyWriterRequest(c, *number, item.Request, comments)
			receipt = publishEvidenceJournal(c, *number, item.Request, receipt)
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
	output, err := ghOutput("issue", "list", "--repo", c.Owner+"/"+c.Repository, "--state", "all", "--limit", "100", "--json", "number")
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
			receipt := applyWriterRequest(c, number, item.Request, comments)
			rejected = append(rejected, publishEvidenceJournal(c, number, item.Request, receipt))
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
	if apply {
		repaired, err := retryPendingWikiJournals(c, number, comments)
		if err != nil {
			return receipted, 0, err
		}
		receipted += repaired
	}
	state, err := loadLiveWork(c, number)
	if err != nil {
		return receipted, 0, err
	}
	if state.IssueState == "CLOSED" && !terminalIssueStatus(state.Status) && hasWriterReceipt(comments) {
		if !apply {
			return receipted, 0, nil
		}
		if err := client.SetIssueState(context.Background(), c.Owner, c.Repository, number, "open"); err != nil {
			return receipted, 0, fmt.Errorf("reopen prematurely closed Issue: %w", err)
		}
		requestID := "premature-close-" + state.IssueID + "-" + state.UpdatedAt
		body, err := writer.RenderReceipt(writer.Receipt{RequestID: requestID, Result: "accepted", Detail: "manual close denied; Issue reopened because completion evidence is incomplete", At: time.Now().UTC()})
		if err != nil {
			return receipted, 0, err
		}
		if _, err := client.CreateComment(context.Background(), c.Owner, c.Repository, number, body); err != nil {
			return receipted, 0, err
		}
		return receipted + 1, 0, nil
	}
	if state.IssueState == "CLOSED" {
		return receipted, 0, nil
	}
	if apply {
		if err := initializeProjectFields(c, number, state); err != nil {
			return receipted, 0, err
		}
		state, err = loadLiveWork(c, number)
		if err != nil {
			return receipted, 0, err
		}
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

func terminalIssueStatus(status string) bool {
	return status == "Done" || status == "Cancelled" || status == "Archived"
}

func hasWriterReceipt(comments []github.Comment) bool {
	for _, comment := range comments {
		if _, err := writer.ParseReceipt(comment.Body); err == nil {
			return true
		}
	}
	return false
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
	Kind       string
	Area       string
	Priority   string
	Proof      string
}

type liveBlocker struct{ ID, State string }

func applyWriterRequest(c model.Config, number int, request writer.Request, comments []github.Comment) writer.Receipt {
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
	if request.Action == "claim" || (request.Action == "status" && request.Status == "Done") {
		for _, blocker := range state.Blockers {
			if blocker.State != "CLOSED" {
				return rejectedReceipt(request, fmt.Errorf("unresolved blocker %s prevents %s", blocker.ID, request.Action))
			}
		}
	}
	if request.Action == "status" && request.Status == "Done" {
		if !acceptedEvidence(comments) {
			return rejectedReceipt(request, errors.New("Done requires a prior accepted evidence receipt"))
		}
		if c.WikiMode == "journal" && pendingWikiJournal(comments) {
			return rejectedReceipt(request, errors.New("Done requires generated Wiki journal to be synchronized"))
		}
	}
	if request.Action == "pr.link" {
		if err := validateReviewPR(request.PR); err != nil {
			return rejectedReceipt(request, err)
		}
	}
	if request.Action == "evidence.submit" {
		if err := validateMergedEvidencePR(request.PR, request.Evidence.FinalSHA); err != nil {
			return rejectedReceipt(request, err)
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
	if next.Status == "Done" {
		if err := github.NewClient().SetIssueState(context.Background(), c.Owner, c.Repository, number, "closed"); err != nil {
			return rejectedReceipt(request, fmt.Errorf("close completed Issue: %w", err))
		}
	}
	receipt := writer.Receipt{RequestID: request.ID, Fingerprint: actual, Result: "accepted", Detail: "lifecycle state changed to " + next.Status, At: time.Now().UTC()}
	if request.Action == "evidence.submit" {
		receipt.Detail = "evidence recorded; lifecycle state changed to Evidence pending"
		receipt.Evidence = request.Evidence
	}
	if next.Status == "Done" {
		receipt.Detail = "completion evidence verified; lifecycle state changed to Done and Issue closed"
	}
	return receipt
}

func acceptedEvidence(comments []github.Comment) bool {
	bodies := make([]string, 0, len(comments))
	for _, comment := range comments {
		bodies = append(bodies, comment.Body)
	}
	return writer.HasAcceptedEvidence(bodies)
}

func pendingWikiJournal(comments []github.Comment) bool {
	for _, comment := range comments {
		receipt, err := writer.ParseReceipt(comment.Body)
		if err == nil && receipt.Result == "accepted" && receipt.Evidence != nil && strings.Contains(receipt.Detail, "Wiki journal pending") {
			return true
		}
	}
	return false
}

func publishEvidenceJournal(c model.Config, number int, request writer.Request, receipt writer.Receipt) writer.Receipt {
	if c.WikiMode != "journal" || receipt.Result != "accepted" || receipt.Evidence == nil {
		return receipt
	}
	entry := journal.Entry{RequestID: receipt.RequestID, Date: receipt.At.UTC().Format("2006-01-02"), Issue: fmt.Sprintf("#%d", number), PR: request.PR, Outcome: receipt.Detail, Proof: receipt.Evidence.Criteria, Boundary: receipt.Evidence.Boundary, NextBlocker: "None"}
	published, err := journal.PublishWiki(context.Background(), c.Owner, c.Repository, github.NewClient().Token, entry)
	if err != nil {
		receipt.Detail += "; Wiki journal pending: " + strings.ReplaceAll(err.Error(), "\n", " ")
		return receipt
	}
	if published {
		receipt.Detail += "; Wiki journal appended"
	}
	return receipt
}

func retryPendingWikiJournals(c model.Config, number int, comments []github.Comment) (int, error) {
	if c.WikiMode != "journal" {
		return 0, nil
	}
	client := github.NewClient()
	repaired := 0
	for _, comment := range comments {
		receipt, err := writer.ParseReceipt(comment.Body)
		if err != nil || receipt.Result != "accepted" || receipt.Evidence == nil || !strings.Contains(receipt.Detail, "Wiki journal pending") {
			continue
		}
		repairID := "wiki-retry-" + receipt.RequestID
		if hasReceipt(comments, repairID) {
			continue
		}
		entry := journal.Entry{RequestID: receipt.RequestID, Date: receipt.At.UTC().Format("2006-01-02"), Issue: fmt.Sprintf("#%d", number), Outcome: "evidence journal repair", Proof: receipt.Evidence.Criteria, Boundary: receipt.Evidence.Boundary, NextBlocker: "None"}
		if _, err := journal.PublishWiki(context.Background(), c.Owner, c.Repository, client.Token, entry); err != nil {
			continue
		}
		body, err := writer.RenderReceipt(writer.Receipt{RequestID: repairID, Result: "accepted", Detail: "generated Wiki journal repaired for evidence request " + receipt.RequestID, At: time.Now().UTC()})
		if err != nil {
			return repaired, err
		}
		if _, err := client.CreateComment(context.Background(), c.Owner, c.Repository, number, body); err != nil {
			return repaired, err
		}
		repaired++
	}
	return repaired, nil
}

func hasReceipt(comments []github.Comment, requestID string) bool {
	for _, comment := range comments {
		receipt, err := writer.ParseReceipt(comment.Body)
		if err == nil && receipt.RequestID == requestID {
			return true
		}
	}
	return false
}

func validateReviewPR(pr string) error {
	if pr == "" {
		return errors.New("PR link requires pull request URL")
	}
	output, err := exec.Command("gh", "pr", "view", pr, "--json", "state,url").Output()
	if err != nil {
		return fmt.Errorf("read pull request: %w", err)
	}
	var result struct {
		State string `json:"state"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return err
	}
	if result.URL != pr || result.State != "OPEN" {
		return errors.New("PR link requires an open canonical pull request URL")
	}
	return nil
}

func validateMergedEvidencePR(pr, finalSHA string) error {
	if pr == "" || finalSHA == "" {
		return errors.New("evidence requires PR URL and final SHA")
	}
	output, err := exec.Command("gh", "pr", "view", pr, "--json", "state,url,mergeCommit").Output()
	if err != nil {
		return fmt.Errorf("read pull request: %w", err)
	}
	var result struct {
		State       string `json:"state"`
		URL         string `json:"url"`
		MergeCommit struct {
			OID string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return err
	}
	if result.URL != pr || result.State != "MERGED" {
		return errors.New("evidence requires merged canonical pull request URL")
	}
	if result.MergeCommit.OID != finalSHA {
		return fmt.Errorf("evidence final SHA %s does not match merged PR head %s", finalSHA, result.MergeCommit.OID)
	}
	return nil
}

func fingerprintLiveWork(state liveWork) (string, error) {
	return writer.StateFingerprint(liveWorkFingerprint(state))
}

func liveWorkFingerprint(state liveWork) writer.Fingerprint {
	// GitHub updates Issue.updatedAt when gfd posts its durable request comment.
	// Including that value makes every request stale before the Writer can read
	// it. The fingerprint therefore covers authoritative graph and Project state,
	// while request parsing separately rejects edited request comments.
	fingerprint := writer.Fingerprint{IssueID: state.IssueID, IssueState: state.IssueState, ParentID: state.ParentID, Project: map[string]string{"status": state.Status, "lease_holder": state.Holder, "lease_expires": state.Expiry, "branch": state.Branch}}
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
	output, err := ghOutput("api", "graphql", "-f", "query="+query)
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
			case "Kind":
				state.Kind = value.Name
			case "Area":
				state.Area = value.Name
			case "Priority":
				state.Priority = value.Name
			case "Proof":
				state.Proof = value.Name
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
		if !value.text {
			option, ok := field.Options[value.value]
			if !ok {
				return fmt.Errorf("Project field %s has no option %q", value.name, value.value)
			}
			if err := setProjectFieldValue(c.Project.ID, itemID, field.ID, option, false); err != nil {
				return fmt.Errorf("update %s: %w", value.name, err)
			}
			continue
		}
		if err := setProjectFieldValue(c.Project.ID, itemID, field.ID, value.value, true); err != nil {
			return fmt.Errorf("update %s: %w", value.name, err)
		}
	}
	return nil
}

// initializeProjectFields fills only blank bootstrap fields. Existing values are
// never overwritten; labels remain source for Kind and Area classification.
func initializeProjectFields(c model.Config, number int, state liveWork) error {
	output, err := ghOutput("issue", "view", fmt.Sprint(number), "--repo", c.Owner+"/"+c.Repository, "--json", "labels")
	if err != nil {
		return err
	}
	var result struct {
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return err
	}
	kind, area := "", ""
	for _, label := range result.Labels {
		if strings.HasPrefix(label.Name, "kind:") {
			value := strings.TrimPrefix(label.Name, "kind:")
			kind = strings.ToUpper(value[:1]) + value[1:]
		}
		if strings.HasPrefix(label.Name, "area:") {
			value := strings.TrimPrefix(label.Name, "area:")
			area = strings.ToUpper(value[:1]) + value[1:]
		}
	}
	if kind == "" {
		return errors.New("cannot initialize Project Kind without kind label")
	}
	if area == "" {
		if output, err := exec.Command("gh", "issue", "edit", fmt.Sprint(number), "--repo", c.Owner+"/"+c.Repository, "--add-label", "area:stable").CombinedOutput(); err != nil {
			return fmt.Errorf("initialize area label: %s", strings.TrimSpace(string(output)))
		}
		area = "Stable"
	}
	values := map[string]string{}
	if state.Status == "" {
		values["Status"] = "Backlog"
	}
	if state.Kind == "" {
		values["Kind"] = kind
	}
	if state.Area == "" {
		values["Area"] = area
	}
	if state.Priority == "" {
		values["Priority"] = "P2"
	}
	if state.Proof == "" {
		values["Proof"] = "Not started"
	}
	if len(values) == 0 {
		return nil
	}
	fields, err := configuredProjectFields(c)
	if err != nil {
		return err
	}
	for name, value := range values {
		field, ok := fields[name]
		if !ok || field.Options[value] == "" {
			return fmt.Errorf("configured Project lacks %s option %q", name, value)
		}
		if err := setProjectFieldValue(c.Project.ID, state.ItemID, field.ID, field.Options[value], false); err != nil {
			return fmt.Errorf("initialize %s: %w", name, err)
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
	// Resolve fields from the configured Project node instead of `gh project
	// field-list --owner`. The latter resolves a user account first, which is
	// unavailable to a least-privilege Writer token even when it can mutate the
	// Project by ID.
	query := fmt.Sprintf(`query { node(id:%q) { ... on ProjectV2 { fields(first:100) { nodes {
... on ProjectV2Field { id name }
... on ProjectV2SingleSelectField { id name options { id name } }
} } } } }`, c.Project.ID)
	output, err := ghOutput("api", "graphql", "-f", "query="+query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data struct {
			Node struct {
				Fields struct {
					Nodes []struct {
						ID      string `json:"id"`
						Name    string `json:"name"`
						Options []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"options"`
					} `json:"nodes"`
				} `json:"fields"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, err
	}
	fields := make(map[string]projectField, len(response.Data.Node.Fields.Nodes))
	for _, raw := range response.Data.Node.Fields.Nodes {
		field := projectField{ID: raw.ID, Options: map[string]string{}}
		for _, option := range raw.Options {
			field.Options[option.Name] = option.ID
		}
		fields[raw.Name] = field
	}
	return fields, nil
}

// ghOutput preserves GitHub CLI diagnostics. Writer reconciliation must expose
// an authorization or schema failure rather than only returning exit status 1.
func ghOutput(args ...string) ([]byte, error) {
	output, err := exec.Command("gh", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return output, nil
}

// setProjectFieldValue uses the Project V2 GraphQL mutations directly. The gh
// project's item-edit command asks for unrelated read:org and read:discussion
// scopes before issuing its API call, while the Writer needs only Project
// mutation authority for this configured Project ID.
func setProjectFieldValue(projectID, itemID, fieldID, value string, text bool) error {
	var mutation string
	if text && value == "" {
		mutation = fmt.Sprintf(`mutation { clearProjectV2ItemFieldValue(input:{projectId:%q,itemId:%q,fieldId:%q}) { projectV2Item { id } } }`, projectID, itemID, fieldID)
	} else if text {
		mutation = fmt.Sprintf(`mutation { updateProjectV2ItemFieldValue(input:{projectId:%q,itemId:%q,fieldId:%q,value:{text:%q}}) { projectV2Item { id } } }`, projectID, itemID, fieldID, value)
	} else {
		mutation = fmt.Sprintf(`mutation { updateProjectV2ItemFieldValue(input:{projectId:%q,itemId:%q,fieldId:%q,value:{singleSelectOptionId:%q}}) { projectV2Item { id } } }`, projectID, itemID, fieldID, value)
	}
	if _, err := ghOutput("api", "graphql", "-f", "query="+mutation); err != nil {
		return err
	}
	return nil
}
