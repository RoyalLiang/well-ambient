package agenda

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"well-ambient/internal/db"
)

// DecisionRequest holds the payload for saving a meeting decision
type DecisionRequest struct {
	TaskID   string `json:"task_id"`
	Action   string `json:"action"` // reassign, suspend, reschedule
	Payload  struct {
		Assignee string `json:"assignee"`
		DueDate  string `json:"due_date"`
		Note     string `json:"note"`
	} `json:"payload"`
	Operator string `json:"operator"`
}

// HandleGetAgendaSummary returns a diagnosis of active red-zone items
func HandleGetAgendaSummary(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var allTasks []db.TaskTelemetry
	// Query all tasks (including done, for generating history auto decisions)
	if err := db.DB.Find(&allTasks).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query tasks: %v", err), http.StatusInternalServerError)
		return
	}

	items := EvaluateActiveTasks(allTasks)

	// Calculate counts from active tasks (status != done)
	redZoneCount := 0
	activeBugCount := 0
	activeTaskCount := 0

	for _, item := range items {
		if item.RiskLevel == "critical" {
			redZoneCount++
		}
	}

	for _, t := range allTasks {
		if strings.ToLower(t.Status) != "done" {
			if t.IssueType == "bug" {
				activeBugCount++
			} else {
				activeTaskCount++
			}
		}
	}

	// Generate ambient auto actions flow
	autoDecisions := GenerateAutonomousDecisions(allTasks)

	response := map[string]interface{}{
		"total_active_tasks": activeBugCount + activeTaskCount,
		"active_bug_count":   activeBugCount,
		"active_task_count":  activeTaskCount,
		"red_zone_count":     redZoneCount,
		"agenda_items":       items,
		"auto_decisions":     autoDecisions,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding agenda summary: %v", err)
	}
}

// HandlePostAgendaDecision updates a task telemetry based on the meeting decision
func HandlePostAgendaDecision(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var req DecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TaskID == "" || req.Action == "" {
		http.Error(w, "Missing task_id or action", http.StatusBadRequest)
		return
	}

	var task db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", req.TaskID).First(&task).Error; err != nil {
		http.Error(w, fmt.Sprintf("Task %s not found", req.TaskID), http.StatusNotFound)
		return
	}

	now := time.Now()
	timestampStr := now.Format("2006-01-02 15:04:05")
	var logDesc string

	switch req.Action {
	case "reassign":
		if req.Payload.Assignee == "" {
			http.Error(w, "Missing assignee in payload", http.StatusBadRequest)
			return
		}
		oldAssignee := task.Assignee
		task.Assignee = req.Payload.Assignee
		logDesc = fmt.Sprintf("[%s] %s 调停干预：将指派人从 [%s] 转派给 [%s]。备注：%s", timestampStr, req.Operator, oldAssignee, task.Assignee, req.Payload.Note)

	case "suspend":
		oldStatus := task.Status
		task.Status = "backlog"
		logDesc = fmt.Sprintf("[%s] %s 调停干预：将任务状态从 [%s] 挂起并放回 [待办中]。备注：%s", timestampStr, req.Operator, oldStatus, req.Payload.Note)

	case "reschedule":
		if req.Payload.DueDate == "" {
			http.Error(w, "Missing due_date in payload", http.StatusBadRequest)
			return
		}
		parsedTime, err := time.Parse(time.RFC3339, req.Payload.DueDate)
		if err != nil {
			parsedTime, err = time.Parse("2006-01-02", req.Payload.DueDate)
			if err != nil {
				http.Error(w, "Invalid due_date format, must be RFC3339 or YYYY-MM-DD", http.StatusBadRequest)
				return
			}
		}
		task.DueDate = &parsedTime
		logDesc = fmt.Sprintf("[%s] %s 调停干预：调整截止时间为 %s。备注：%s", timestampStr, req.Operator, parsedTime.Format("2006-01-02"), req.Payload.Note)

	default:
		http.Error(w, fmt.Sprintf("Unknown action: %s", req.Action), http.StatusBadRequest)
		return
	}

	// Append to decision logs
	if task.DecisionLogs != "" {
		task.DecisionLogs = task.DecisionLogs + "\n" + logDesc
	} else {
		task.DecisionLogs = logDesc
	}

	task.LastUpdate = now

	// Save back to db
	if err := db.DB.Save(&task).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to save task: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"message":      "Decision saved and synced successfully",
		"updated_task": task,
	})
}
