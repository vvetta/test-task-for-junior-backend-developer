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
	TaskKindSingle TaskKind = "single"
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
	RecurrenceType RecurrenceType
	StartDate time.Time
	EndDate *time.Time
	TimeZone string
	EveryNDays *int
	DayOfMonth *int
	DayParity DayParity
	SpecificDates []time.Time
	LastGeneratedFor *time.Time
}

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Kind 	TaskKind
	ScheduledFor *time.Time
	ParentTaskID *int64
	IsActive bool
	Recurrence *RecurrenceRule
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
