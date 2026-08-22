package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

// DeliveryDirectoryResponse is the canonical option directory for delivery
// filters and ownership forms. Authorization groups never define delivery
// ownership; configured Jira core members do.
type DeliveryDirectoryResponse struct {
	Assignees []ExecutionAssigneeOptionDTO `json:"assignees"`
	Projects  []ExecutionProjectOptionDTO  `json:"projects"`
}

func (s *Server) handleGetDeliveryDirectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	directory, err := s.loadDeliveryDirectory()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load delivery directory: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(directory)
}

func (s *Server) loadDeliveryDirectory() (DeliveryDirectoryResponse, error) {
	visibility, users, err := s.loadCoreMemberVisibility()
	if err != nil {
		return DeliveryDirectoryResponse{}, err
	}
	projects, err := s.availableProjectPreferenceOptions()
	if err != nil {
		return DeliveryDirectoryResponse{}, err
	}

	projectOptions := make([]ExecutionProjectOptionDTO, 0, len(projects))
	for _, project := range projects {
		projectOptions = append(projectOptions, ExecutionProjectOptionDTO{
			ProjectKey:  project.ProjectKey,
			ProjectName: project.ProjectName,
		})
	}

	return DeliveryDirectoryResponse{
		Assignees: s.deliveryAssigneeOptions(visibility, users),
		Projects:  projectOptions,
	}, nil
}

func (s *Server) deliveryAssigneeOptions(visibility coreMemberVisibility, users []userdb.User) []ExecutionAssigneeOptionDTO {
	optionsByValue := make(map[string]ExecutionAssigneeOptionDTO)
	addOption := func(value, department string) {
		value = strings.TrimSpace(value)
		if value == "" || value == "-" || value == "未指派" || strings.EqualFold(value, "unassigned") {
			return
		}
		key := strings.ToLower(value)
		option := optionsByValue[key]
		option.Value = value
		option.Label = value
		if option.Department == "" && strings.TrimSpace(department) != "" && department != "未分配" {
			option.Department = strings.TrimSpace(department)
		}
		optionsByValue[key] = option
	}
	addAliases := func(value string, aliases ...string) {
		key := strings.ToLower(strings.TrimSpace(value))
		option, exists := optionsByValue[key]
		if !exists {
			return
		}
		seen := make(map[string]struct{}, len(option.Aliases)+len(aliases))
		for _, alias := range option.Aliases {
			seen[strings.ToLower(alias)] = struct{}{}
		}
		for _, alias := range aliases {
			alias = strings.TrimSpace(alias)
			aliasKey := strings.ToLower(alias)
			if alias == "" || aliasKey == key {
				continue
			}
			if _, exists := seen[aliasKey]; exists {
				continue
			}
			seen[aliasKey] = struct{}{}
			option.Aliases = append(option.Aliases, alias)
		}
		optionsByValue[key] = option
	}

	directory := newKPIUserDirectory(users)
	if visibility.filter.enabled {
		for _, configured := range s.configuredKPICoreMembers() {
			identity := directory.resolve(configured)
			if identity.Name == "" {
				addOption(configured, "")
				continue
			}
			addOption(identity.Name, identity.Department)
			addAliases(identity.Name, configured, identity.Username)
		}
	} else {
		// Local and test environments without a configured member boundary stay
		// usable. Once sync_users/custom JQL is configured, this fallback is off.
		for _, user := range users {
			identity := kpiIdentityFromUser(user)
			addOption(identity.Name, identity.Department)
		}
		var owners []string
		if err := db.DB.Model(&db.TaskTelemetry{}).
			Distinct("assignee").
			Where("TRIM(assignee) <> ''").
			Order("assignee ASC").
			Limit(5000).
			Pluck("assignee", &owners).Error; err == nil {
			for _, owner := range owners {
				identity := directory.resolve(owner)
				if identity.Name != "" {
					addOption(identity.Name, identity.Department)
				} else {
					addOption(owner, "")
				}
			}
		}
	}

	options := make([]ExecutionAssigneeOptionDTO, 0, len(optionsByValue))
	for _, option := range optionsByValue {
		options = append(options, option)
	}
	sort.Slice(options, func(i, j int) bool {
		return strings.ToLower(options[i].Label) < strings.ToLower(options[j].Label)
	})
	return options
}
