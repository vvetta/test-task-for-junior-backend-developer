package task

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Service struct {
	repo        Repository
	now         func() time.Time
	horizonDays int
}

func NewService(repo Repository) *Service {
	return &Service{
		repo:        repo,
		now:         func() time.Time { return time.Now().UTC() },
		horizonDays: 60,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	normalized, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	model := &taskdomain.Task{
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		Kind:        taskdomain.TaskKindSingle,
		IsActive:    true,
	}

	if normalized.Recurrence != nil {
		model.Kind = taskdomain.TaskKindTemplate
		model.Recurrence = toDomainRule(*normalized.Recurrence)
	}

	now := s.now()
	model.CreatedAt = now
	model.UpdatedAt = now

	created, err := s.repo.Create(ctx, model)
	if err != nil {
		return nil, err
	}

	if created.Kind == taskdomain.TaskKindTemplate {
		if err := s.generateForTemplate(ctx, *created, s.generationWindow()); err != nil {
			return nil, err
		}
		return s.repo.GetByID(ctx, created.ID)
	}

	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	normalized, err := validateUpdateInput(*existing, input)
	if err != nil {
		return nil, err
	}

	existing.Title = normalized.Title
	existing.Description = normalized.Description
	existing.Status = normalized.Status
	existing.UpdatedAt = s.now()

	if existing.Kind == taskdomain.TaskKindTemplate {
		existing.Recurrence = toDomainRule(*normalized.Recurrence)
		existing.IsActive = true
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}

	if updated.Kind == taskdomain.TaskKindTemplate && updated.IsActive {
		if err := s.generateForTemplate(ctx, *updated, s.generationWindow()); err != nil {
			return nil, err
		}
		return s.repo.GetByID(ctx, updated.ID)
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if existing.Kind == taskdomain.TaskKindTemplate {
		return s.repo.DeactivateTemplate(ctx, id)
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]taskdomain.Task, error) {
	switch filter {
	case "", ListFilterWork:
		return s.repo.List(ctx, ListFilterWork)
	case ListFilterTemplate, ListFilterAll:
		return s.repo.List(ctx, filter)
	default:
		return nil, fmt.Errorf("%w: invalid list filter", ErrInvalidInput)
	}
}

func (s *Service) GeneratePending(ctx context.Context) error {
	templates, err := s.repo.ListActiveTemplates(ctx)
	if err != nil {
		return err
	}

	until := s.generationWindow()
	for i := range templates {
		if err := s.generateForTemplate(ctx, templates[i], until); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) generateForTemplate(ctx context.Context, tpl taskdomain.Task, until time.Time) error {
	if tpl.Kind != taskdomain.TaskKindTemplate || tpl.Recurrence == nil || !tpl.IsActive {
		return nil
	}

	from := maxDate(normalizeDate(tpl.Recurrence.StartDate), currentDateInZone(s.now(), tpl.Recurrence.TimeZone))
	to := until
	if tpl.Recurrence.EndDate != nil && tpl.Recurrence.EndDate.Before(to) {
		to = normalizeDate(*tpl.Recurrence.EndDate)
	}

	if from.After(to) {
		return nil
	}

	dates := buildDates(*tpl.Recurrence, from, to)
	now := s.now()

	for _, date := range dates {
		scheduledFor := date
		parentID := tpl.ID

		instance := &taskdomain.Task{
			Title:        tpl.Title,
			Description:  tpl.Description,
			Status:       taskdomain.StatusNew,
			Kind:         taskdomain.TaskKindInstance,
			ScheduledFor: &scheduledFor,
			ParentTaskID: &parentID,
			IsActive:     true,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := s.repo.CreateGeneratedInstance(ctx, instance); err != nil {
			return err
		}
	}

	return s.repo.SetTemplateLastGeneratedFor(ctx, tpl.ID, to)
}

func (s *Service) generationWindow() time.Time {
	return normalizeDate(s.now().AddDate(0, 0, s.horizonDays))
}

func validateCreateInput(input CreateInput) (CreateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return CreateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}

	if !input.Status.Valid() {
		return CreateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	if input.Recurrence != nil {
		normalized, err := validateRecurrence(*input.Recurrence)
		if err != nil {
			return CreateInput{}, err
		}
		input.Recurrence = &normalized
	}

	return input, nil
}

func validateUpdateInput(existing taskdomain.Task, input UpdateInput) (UpdateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return UpdateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if !input.Status.Valid() {
		return UpdateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	switch existing.Kind {
	case taskdomain.TaskKindSingle, taskdomain.TaskKindInstance:
		if input.Recurrence != nil {
			return UpdateInput{}, fmt.Errorf("%w: converting existing task to template is not supported", ErrInvalidInput)
		}
	case taskdomain.TaskKindTemplate:
		if input.Recurrence == nil {
			return UpdateInput{}, fmt.Errorf("%w: recurrence is required for template update", ErrInvalidInput)
		}

		normalized, err := validateRecurrence(*input.Recurrence)
		if err != nil {
			return UpdateInput{}, err
		}
		input.Recurrence = &normalized
	}

	return input, nil
}

func validateRecurrence(input RecurrenceInput) (RecurrenceInput, error) {
	if !input.Type.Valid() {
		return RecurrenceInput{}, fmt.Errorf("%w: invalid recurrence type", ErrInvalidInput)
	}

	input.TimeZone = strings.TrimSpace(input.TimeZone)
	if input.TimeZone == "" {
		input.TimeZone = "UTC"
	}

	if _, err := time.LoadLocation(input.TimeZone); err != nil {
		return RecurrenceInput{}, fmt.Errorf("%w: invalid time_zone", ErrInvalidInput)
	}

	input.SpecificDates = normalizeDateSlice(input.SpecificDates)
	sort.Slice(input.SpecificDates, func(i, j int) bool { return input.SpecificDates[i].Before(input.SpecificDates[j]) })
	input.SpecificDates = uniqueDates(input.SpecificDates)

	if input.Type == taskdomain.RecurrenceSpecificDates {
		if len(input.SpecificDates) == 0 {
			return RecurrenceInput{}, fmt.Errorf("%w: specific_dates are required", ErrInvalidInput)
		}

		if input.StartDate == nil {
			first := input.SpecificDates[0]
			input.StartDate = &first
		}
		if input.EndDate == nil {
			last := input.SpecificDates[len(input.SpecificDates)-1]
			input.EndDate = &last
		}
	}

	if input.StartDate == nil {
		return RecurrenceInput{}, fmt.Errorf("%w: start_date is required", ErrInvalidInput)
	}

	start := normalizeDate(*input.StartDate)
	input.StartDate = &start

	if input.EndDate != nil {
		end := normalizeDate(*input.EndDate)
		input.EndDate = &end
		if end.Before(start) {
			return RecurrenceInput{}, fmt.Errorf("%w: end_date must be on or after start_date", ErrInvalidInput)
		}
	}

	switch input.Type {
	case taskdomain.RecurrenceDaily:
		if input.EveryNDays == nil || *input.EveryNDays < 1 {
			return RecurrenceInput{}, fmt.Errorf("%w: every_n_days must be >= 1", ErrInvalidInput)
		}

		if input.DayOfMonth != nil || input.Parity != nil || len(input.SpecificDates) > 0 {
			return RecurrenceInput{}, fmt.Errorf("%w: only daily settings are allowed", ErrInvalidInput)
		}
	case taskdomain.RecurrenceMonthly:
		if input.DayOfMonth == nil || *input.DayOfMonth < 1 || *input.DayOfMonth > 30 {
			return RecurrenceInput{}, fmt.Errorf("%w: day_of_month must be 1..30", ErrInvalidInput)
		}

		if input.EveryNDays != nil || input.Parity != nil || len(input.SpecificDates) > 0 {
			return RecurrenceInput{}, fmt.Errorf("%w: only monthly settings are allowed", ErrInvalidInput)
		}
	case taskdomain.RecurrenceDayParity:
		if input.Parity == nil || !input.Parity.Valid() {
			return RecurrenceInput{}, fmt.Errorf("%w: parity must be even or odd", ErrInvalidInput)
		}

		if input.EveryNDays != nil || input.DayOfMonth != nil || len(input.SpecificDates) > 0 {
			return RecurrenceInput{}, fmt.Errorf("%w: only parity settings are allowed", ErrInvalidInput)
		}
	case taskdomain.RecurrenceSpecificDates:
		if input.EveryNDays != nil || input.DayOfMonth != nil || input.Parity != nil {
			return RecurrenceInput{}, fmt.Errorf("%w: only specific_dates settings are allowed", ErrInvalidInput)
		}
	}

	return input, nil
}

func toDomainRule(input RecurrenceInput) *taskdomain.RecurrenceRule {
	if input.StartDate == nil {
		return nil
	}

	return &taskdomain.RecurrenceRule{
		RecurrenceType: input.Type,
		StartDate:      *input.StartDate,
		EndDate:        input.EndDate,
		TimeZone:       input.TimeZone,
		EveryNDays:     input.EveryNDays,
		DayOfMonth:     input.DayOfMonth,
		DayParity:      input.Parity,
		SpecificDates:  input.SpecificDates,
	}
}

func buildDates(rule taskdomain.RecurrenceRule, from, to time.Time) []time.Time {
	switch rule.RecurrenceType {
	case taskdomain.RecurrenceDaily:
		return buildDailyDates(rule, from, to)
	case taskdomain.RecurrenceMonthly:
		return buildMonthlyDates(rule, from, to)
	case taskdomain.RecurrenceSpecificDates:
		return buildSpecificDates(rule, from, to)
	case taskdomain.RecurrenceDayParity:
		return buildParityDates(rule, from, to)
	default:
		return nil
	}
}

func buildDailyDates(rule taskdomain.RecurrenceRule, from, to time.Time) []time.Time {
	if rule.EveryNDays == nil {
		return nil
	}

	result := make([]time.Time, 0)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		diff := int(d.Sub(normalizeDate(rule.StartDate)).Hours() / 24)
		if diff >= 0 && diff%*rule.EveryNDays == 0 {
			result = append(result, d)
		}
	}

	return result
}

func buildMonthlyDates(rule taskdomain.RecurrenceRule, from, to time.Time) []time.Time {
	if rule.DayOfMonth == nil {
		return nil
	}

	result := make([]time.Time, 0)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if d.Day() == *rule.DayOfMonth {
			result = append(result, d)
		}
	}

	return result
}

func buildSpecificDates(rule taskdomain.RecurrenceRule, from, to time.Time) []time.Time {
	result := make([]time.Time, 0, len(rule.SpecificDates))

	for _, d := range rule.SpecificDates {
		normalized := normalizeDate(d)
		if normalized.Before(from) || normalized.After(to) {
			continue
		}

		result = append(result, normalized)
	}

	return result
}

func buildParityDates(rule taskdomain.RecurrenceRule, from, to time.Time) []time.Time {
	if rule.DayParity == nil {
		return nil
	}

	result := make([]time.Time, 0)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		switch *rule.DayParity {
		case taskdomain.DayParityEven:
			if d.Day()%2 == 0 {
				result = append(result, d)
			}
		case taskdomain.DayParityOdd:
			if d.Day()%2 != 0 {
				result = append(result, d)
			}
		}
	}

	return result
}

func normalizeDate(value time.Time) time.Time {
	return time.Date(
		value.Year(),
		value.Month(),
		value.Day(),
		0, 0, 0, 0,
		time.UTC)
}

func normalizeDateSlice(values []time.Time) []time.Time {
	result := make([]time.Time, 0, len(values))

	for _, v := range values {
		result = append(result, normalizeDate(v))
	}

	return result
}

func uniqueDates(values []time.Time) []time.Time {
	if len(values) == 0 {
		return values
	}

	result := []time.Time{values[0]}
	for i := 1; i < len(values); i++ {
		if values[i].Equal(values[i-1]) {
			continue
		}
		result = append(result, values[i])
	}
	return result
}

func currentDateInZone(now time.Time, zone string) time.Time {
	loc, err := time.LoadLocation(zone)

	if err != nil {
		return normalizeDate(now.UTC())
	}

	localNow := now.In(loc)

	return time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		0, 0, 0, 0,
		time.UTC)
}

func maxDate(left, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}
