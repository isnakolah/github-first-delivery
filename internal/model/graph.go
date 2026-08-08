package model

import "fmt"

type Issue struct {
	ID         string   `json:"id"`
	Number     int      `json:"number"`
	Kind       string   `json:"kind"`
	Status     string   `json:"status"`
	ParentID   string   `json:"parent_id"`
	BlockerIDs []string `json:"blocker_ids"`
	Body       string   `json:"body"`
}

func ValidateGraph(issues []Issue) error {
	byID := make(map[string]Issue, len(issues))
	for _, issue := range issues {
		if issue.ID == "" {
			return fmt.Errorf("issue id is required")
		}
		if _, ok := byID[issue.ID]; ok {
			return fmt.Errorf("duplicate issue %s", issue.ID)
		}
		byID[issue.ID] = issue
	}
	for _, issue := range issues {
		if issue.Kind != "Epic" && issue.Status != "Archived" && issue.ParentID == "" {
			return fmt.Errorf("issue #%d has no parent", issue.Number)
		}
		if issue.Kind != "Epic" {
			if err := ValidateWorkContract(issue.Body); err != nil {
				return fmt.Errorf("issue #%d: %w", issue.Number, err)
			}
		}
		for _, blocker := range issue.BlockerIDs {
			if _, ok := byID[blocker]; !ok {
				return fmt.Errorf("issue #%d references unknown blocker %s", issue.Number, blocker)
			}
		}
	}
	for _, issue := range issues {
		if err := assertAcyclic(issue.ID, byID); err != nil {
			return err
		}
	}
	return nil
}

func assertAcyclic(start string, byID map[string]Issue) error {
	seen := map[string]bool{}
	for id := start; id != ""; {
		if seen[id] {
			return fmt.Errorf("parent cycle at issue %s", id)
		}
		seen[id] = true
		issue, ok := byID[id]
		if !ok {
			return fmt.Errorf("unknown parent issue %s", id)
		}
		id = issue.ParentID
	}
	return nil
}
