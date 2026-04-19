package task

import "time"

type Status string
type TaskKind string
type RecurrenceType string
type DayParity string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

const (
	TaskKindSingle   TaskKind = "single"
	TaskKindTemplate TaskKind = "template"
	TaskKindInstance TaskKind = "instance"
)

const (
	RecurrenceDaily         RecurrenceType = "daily"
	RecurrenceMonthly       RecurrenceType = "monthly"
	RecurrenceSpecificDates RecurrenceType = "specific_dates"
	RecurrenceDayParity     RecurrenceType = "day_parity"
)

const (
	DayParityEven DayParity = "even"
	DayParityOdd  DayParity = "odd"
)

type RecurrenceRule struct {
	RecurrenceType   RecurrenceType `json:"type"`
	StartDate        time.Time      `json:"start_date"`
	EndDate          *time.Time     `json:"end_date,omitempty"`
	TimeZone         string         `json:"time_zone"`
	EveryNDays       *int           `json:"every_n_days,omitempty"`
	DayOfMonth       *int           `json:"day_of_month,omitempty"`
	DayParity        *DayParity     `json:"day_parity,omitempty"`
	SpecificDates    []time.Time    `json:"specific_dates,omitempty"`
	LastGeneratedFor *time.Time     `json:"last_generated_for,omitempty"`
}

type Task struct {
	ID           int64           `json:"id"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Status       Status          `json:"status"`
	Kind         TaskKind        `json:"kind"`
	ScheduledFor *time.Time      `json:"scheduled_for,omitempty"`
	ParentTaskID *int64          `json:"parent_task_id,omitempty"`
	IsActive     bool            `json:"is_active"`
	Recurrence   *RecurrenceRule `json:"recurrence,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}

func (k TaskKind) Valid() bool {
	switch k {
	case TaskKindSingle, TaskKindTemplate, TaskKindInstance:
		return true
	default:
		return false
	}
}

func (t RecurrenceType) Valid() bool {
	switch t {
	case RecurrenceDaily, RecurrenceMonthly, RecurrenceSpecificDates, RecurrenceDayParity:
		return true
	default:
		return false
	}
}

func (p DayParity) Valid() bool {
	switch p {
	case DayParityEven, DayParityOdd:
		return true
	default:
		return false
	}
}
