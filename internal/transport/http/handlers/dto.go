package handlers

import (
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type recurrenceMutationDTO struct {
	Type          taskdomain.RecurrenceType `json:"type"`
	StartDate     *string                   `json:"start_date,omitempty"`
	EndDate       *string                   `json:"end_date,omitempty"`
	TimeZone      string                    `json:"time_zone,omitempty"`
	EveryNDays    *int                      `json:"every_n_days,omitempty"`
	DayOfMonth    *int                      `json:"day_of_month,omitempty"`
	Parity        *taskdomain.DayParity     `json:"parity,omitempty"`
	SpecificDates []string                  `json:"specific_dates,omitempty"`
}

type taskMutationDTO struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Status      taskdomain.Status      `json:"status"`
	Recurrence  *recurrenceMutationDTO `json:"recurrence,omitempty"`
}

type recurrenceDTO struct {
	Type             taskdomain.RecurrenceType `json:"type"`
	StartDate        string                    `json:"start_date"`
	EndDate          *string                   `json:"end_date,omitempty"`
	TimeZone         string                    `json:"time_zone"`
	EveryNDays       *int                      `json:"every_n_days,omitempty"`
	DayOfMonth       *int                      `json:"day_of_month,omitempty"`
	Parity           *taskdomain.DayParity     `json:"parity,omitempty"`
	SpecificDates    []string                  `json:"specific_dates,omitempty"`
	LastGeneratedFor *string                   `json:"last_generated_for,omitempty"`
}

type taskDTO struct {
	ID           int64               `json:"id"`
	Title        string              `json:"title"`
	Description  string              `json:"description"`
	Status       taskdomain.Status   `json:"status"`
	Kind         taskdomain.TaskKind `json:"kind"`
	ScheduledFor *string             `json:"scheduled_for,omitempty"`
	ParentTaskID *int64              `json:"parent_task_id,omitempty"`
	IsActive     bool                `json:"is_active"`
	Recurrence   *recurrenceDTO      `json:"recurrence,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

func newTaskDTO(task *taskdomain.Task) taskDTO {
	dto := taskDTO{
		ID:           task.ID,
		Title:        task.Title,
		Description:  task.Description,
		Status:       task.Status,
		Kind:         task.Kind,
		ScheduledFor: formatDatePtr(task.ScheduledFor),
		ParentTaskID: task.ParentTaskID,
		IsActive:     task.IsActive,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}

	if task.Recurrence != nil {
		dto.Recurrence = &recurrenceDTO{
			Type:             task.Recurrence.RecurrenceType,
			StartDate:        formatDate(task.Recurrence.StartDate),
			EndDate:          formatDatePtr(task.Recurrence.EndDate),
			TimeZone:         task.Recurrence.TimeZone,
			EveryNDays:       task.Recurrence.EveryNDays,
			DayOfMonth:       task.Recurrence.DayOfMonth,
			Parity:           task.Recurrence.DayParity,
			SpecificDates:    formatDateSlice(task.Recurrence.SpecificDates),
			LastGeneratedFor: formatDatePtr(task.Recurrence.LastGeneratedFor),
		}
	}

	return dto
}

func parseDatePtr(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}

	value, err := time.Parse("2006-01-02", *raw)
	if err != nil {
		return nil, err
	}

	normalized := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
	return &normalized, nil
}

func parseDateSlice(raw []string) ([]time.Time, error) {
	result := make([]time.Time, 0, len(raw))
	for _, item := range raw {
		value, err := time.Parse("2006-01-02", item)
		if err != nil {
			return nil, err
		}
		result = append(result, time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC))
	}

	return result, nil
}

func formatDate(value time.Time) string {
	return value.UTC().Format("2006-01-02")
}

func formatDatePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}

	formatted := value.UTC().Format("2006-01-02")
	return &formatted
}

func formatDateSlice(values []time.Time) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, formatDate(value))
	}

	return result
}
