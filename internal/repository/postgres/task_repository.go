package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
	taskusecase "example.com/taskservice/internal/usecase/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const insertTask = `
        INSERT INTO tasks (title, description, status, kind, scheduled_for, parent_task_id, is_active, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        RETURNING id, title, description, status, kind, scheduled_for, parent_task_id, is_active, created_at, updated_at
    `

	row := tx.QueryRow(ctx, insertTask,
		task.Title,
		task.Description,
		task.Status,
		task.Kind,
		task.ScheduledFor,
		task.ParentTaskID,
		task.IsActive,
		task.CreatedAt,
		task.UpdatedAt,
	)

	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	if task.Kind == taskdomain.TaskKindTemplate && task.Recurrence != nil {
		if err := insertRule(ctx, tx, created.ID, task.Recurrence); err != nil {
			return nil, err
		}
		created.Recurrence = task.Recurrence
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
        SELECT
            t.id,
            t.title,
            t.description,
            t.status,
            t.kind,
            t.scheduled_for,
            t.parent_task_id,
            t.is_active,
            t.created_at,
            t.updated_at,
            rr.recurrence_type,
            rr.start_date,
            rr.end_date,
            rr.time_zone,
            rr.every_n_days,
            rr.day_of_month,
            rr.day_parity,
            rr.specific_dates,
            rr.last_generated_for
        FROM tasks t
        LEFT JOIN task_recurrence_rules rr ON rr.task_id = t.id
        WHERE t.id = $1
    `

	row := r.pool.QueryRow(ctx, query, id)
	model, err := scanTaskWithRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}
		return nil, err
	}
	return model, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const updateTask = `
        UPDATE tasks
        SET title = $1,
            description = $2,
            status = $3,
            is_active = $4,
            updated_at = $5
        WHERE id = $6
        RETURNING id, title, description, status, kind, scheduled_for, parent_task_id, is_active, created_at, updated_at
    `

	row := tx.QueryRow(ctx, updateTask,
		task.Title,
		task.Description,
		task.Status,
		task.IsActive,
		task.UpdatedAt,
		task.ID,
	)

	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}
		return nil, err
	}

	if task.Kind == taskdomain.TaskKindTemplate && task.Recurrence != nil {
		if err := upsertRule(ctx, tx, task.ID, task.Recurrence); err != nil {
			return nil, err
		}
		updated.Recurrence = task.Recurrence
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, task.ID)
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}
	return nil
}

func (r *Repository) DeactivateTemplate(ctx context.Context, id int64) error {
	const query = `
        UPDATE tasks
        SET is_active = FALSE, updated_at = NOW()
        WHERE id = $1 AND kind = 'template'
    `

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}
	return nil
}

func (r *Repository) List(ctx context.Context, filter taskusecase.ListFilter) ([]taskdomain.Task, error) {
	base := `
        SELECT
            t.id,
            t.title,
            t.description,
            t.status,
            t.kind,
            t.scheduled_for,
            t.parent_task_id,
            t.is_active,
            t.created_at,
            t.updated_at,
            rr.recurrence_type,
            rr.start_date,
            rr.end_date,
            rr.time_zone,
            rr.every_n_days,
            rr.day_of_month,
            rr.day_parity,
            rr.specific_dates,
            rr.last_generated_for
        FROM tasks t
        LEFT JOIN task_recurrence_rules rr ON rr.task_id = t.id
    `

	switch filter {
	case taskusecase.ListFilterTemplate:
		base += ` WHERE t.kind = 'template' `
	case taskusecase.ListFilterAll:
		// no filter
	default:
		base += ` WHERE t.kind IN ('single', 'instance') AND t.is_active = TRUE `
	}

	base += ` ORDER BY t.scheduled_for DESC NULLS LAST, t.id DESC `

	rows, err := r.pool.Query(ctx, base)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		item, err := scanTaskWithRule(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *Repository) ListActiveTemplates(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
        SELECT
            t.id,
            t.title,
            t.description,
            t.status,
            t.kind,
            t.scheduled_for,
            t.parent_task_id,
            t.is_active,
            t.created_at,
            t.updated_at,
            rr.recurrence_type,
            rr.start_date,
            rr.end_date,
            rr.time_zone,
            rr.every_n_days,
            rr.day_of_month,
            rr.day_parity,
            rr.specific_dates,
            rr.last_generated_for
        FROM tasks t
        JOIN task_recurrence_rules rr ON rr.task_id = t.id
        WHERE t.kind = 'template' AND t.is_active = TRUE
        ORDER BY t.id DESC
    `

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]taskdomain.Task, 0)
	for rows.Next() {
		item, err := scanTaskWithRule(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repository) CreateGeneratedInstance(ctx context.Context, task *taskdomain.Task) error {
	const query = `
        INSERT INTO tasks (title, description, status, kind, scheduled_for, parent_task_id, is_active, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        ON CONFLICT (parent_task_id, scheduled_for)
        WHERE kind = 'instance' AND parent_task_id IS NOT NULL AND scheduled_for IS NOT NULL
        DO NOTHING
    `

	_, err := r.pool.Exec(ctx, query,
		task.Title,
		task.Description,
		task.Status,
		task.Kind,
		task.ScheduledFor,
		task.ParentTaskID,
		task.IsActive,
		task.CreatedAt,
		task.UpdatedAt,
	)
	return err
}

func (r *Repository) SetTemplateLastGeneratedFor(ctx context.Context, templateID int64, date time.Time) error {
	const query = `
        UPDATE task_recurrence_rules
        SET last_generated_for = $1, updated_at = NOW()
        WHERE task_id = $2
    `

	result, err := r.pool.Exec(ctx, query, date, templateID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}
	return nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		model        taskdomain.Task
		status       string
		kind         string
		scheduledFor *time.Time
		parentTaskID *int64
	)

	if err := scanner.Scan(
		&model.ID,
		&model.Title,
		&model.Description,
		&status,
		&kind,
		&scheduledFor,
		&parentTaskID,
		&model.IsActive,
		&model.CreatedAt,
		&model.UpdatedAt,
	); err != nil {
		return nil, err
	}

	model.Status = taskdomain.Status(status)
	model.Kind = taskdomain.TaskKind(kind)
	model.ScheduledFor = scheduledFor
	model.ParentTaskID = parentTaskID

	return &model, nil
}

func scanTaskWithRule(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		model          taskdomain.Task
		status         string
		kind           string
		scheduledFor   *time.Time
		parentTaskID   *int64
		recurrenceType *string
		startDate      *time.Time
		endDate        *time.Time
		timeZone       *string
		everyNDays     *int
		dayOfMonth     *int
		dayParity      *string
		specificDates  []time.Time
		lastGenerated  *time.Time
	)

	if err := scanner.Scan(
		&model.ID,
		&model.Title,
		&model.Description,
		&status,
		&kind,
		&scheduledFor,
		&parentTaskID,
		&model.IsActive,
		&model.CreatedAt,
		&model.UpdatedAt,
		&recurrenceType,
		&startDate,
		&endDate,
		&timeZone,
		&everyNDays,
		&dayOfMonth,
		&dayParity,
		&specificDates,
		&lastGenerated,
	); err != nil {
		return nil, err
	}

	model.Status = taskdomain.Status(status)
	model.Kind = taskdomain.TaskKind(kind)
	model.ScheduledFor = scheduledFor
	model.ParentTaskID = parentTaskID

	if recurrenceType != nil && startDate != nil && timeZone != nil {
		rule := &taskdomain.RecurrenceRule{
			RecurrenceType:   taskdomain.RecurrenceType(*recurrenceType),
			StartDate:        *startDate,
			EndDate:          endDate,
			TimeZone:         *timeZone,
			EveryNDays:       everyNDays,
			DayOfMonth:       dayOfMonth,
			SpecificDates:    specificDates,
			LastGeneratedFor: lastGenerated,
		}
		if dayParity != nil {
			value := taskdomain.DayParity(*dayParity)
			rule.DayParity = &value
		}
		model.Recurrence = rule
	}

	return &model, nil
}

func insertRule(ctx context.Context, tx pgx.Tx, taskID int64, rule *taskdomain.RecurrenceRule) error {
	const query = `
        INSERT INTO task_recurrence_rules (
            task_id, recurrence_type, start_date, end_date, time_zone,
            every_n_days, day_of_month, day_parity, specific_dates,
            last_generated_for, created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
    `

	_, err := tx.Exec(ctx, query,
		taskID,
		rule.RecurrenceType,
		rule.StartDate,
		rule.EndDate,
		rule.TimeZone,
		rule.EveryNDays,
		rule.DayOfMonth,
		parityToDB(rule.DayParity),
		rule.SpecificDates,
		rule.LastGeneratedFor,
	)
	return err
}

func upsertRule(ctx context.Context, tx pgx.Tx, taskID int64, rule *taskdomain.RecurrenceRule) error {
	const query = `
        INSERT INTO task_recurrence_rules (
            task_id, recurrence_type, start_date, end_date, time_zone,
            every_n_days, day_of_month, day_parity, specific_dates,
            last_generated_for, created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULL, NOW(), NOW())
        ON CONFLICT (task_id)
        DO UPDATE SET
            recurrence_type = EXCLUDED.recurrence_type,
            start_date = EXCLUDED.start_date,
            end_date = EXCLUDED.end_date,
            time_zone = EXCLUDED.time_zone,
            every_n_days = EXCLUDED.every_n_days,
            day_of_month = EXCLUDED.day_of_month,
            day_parity = EXCLUDED.day_parity,
            specific_dates = EXCLUDED.specific_dates,
            last_generated_for = NULL,
            updated_at = NOW()
    `

	_, err := tx.Exec(ctx, query,
		taskID,
		rule.RecurrenceType,
		rule.StartDate,
		rule.EndDate,
		rule.TimeZone,
		rule.EveryNDays,
		rule.DayOfMonth,
		parityToDB(rule.DayParity),
		rule.SpecificDates,
	)
	return err
}

func parityToDB(value *taskdomain.DayParity) *string {
	if value == nil {
		return nil
	}
	v := string(*value)
	return &v
}
