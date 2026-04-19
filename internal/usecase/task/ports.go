package task

import (
	"context"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type ListFilter string

const (
	ListFilterWork     ListFilter = "work"
	ListFilterTemplate ListFilter = "template"
	ListFilterAll      ListFilter = "all"
)

type Repository interface {
	Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter ListFilter) ([]taskdomain.Task, error)

	ListActiveTemplates(ctx context.Context) ([]taskdomain.Task, error)
	CreateGeneratedInstance(ctx context.Context, task *taskdomain.Task) error
	SetTemplateLastGeneratedFor(ctx context.Context, templateID int64, date time.Time) error
	DeactivateTemplate(ctx context.Context, id int64) error
}

type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter ListFilter) ([]taskdomain.Task, error)
}

type CreateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	Recurrence  *RecurrenceInput
}

type UpdateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	Recurrence  *RecurrenceInput
}

type RecurrenceInput struct {
	Type          taskdomain.RecurrenceType
	StartDate     *time.Time
	EndDate       *time.Time
	TimeZone      string
	EveryNDays    *int
	DayOfMonth    *int
	Parity        *taskdomain.DayParity
	SpecificDates []time.Time
}
