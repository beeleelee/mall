package a2a

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	domain "github.com/beeleelee/mall/domain/a2a"
	"github.com/beeleelee/mall/domain/kernel"
)

type nullRawMessage []byte

func (m *nullRawMessage) Scan(src any) error {
	if src == nil {
		*m = nil
		return nil
	}
	*m = src.([]byte)
	return nil
}

func (m nullRawMessage) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return []byte(m), nil
}

func (m *nullRawMessage) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*m = nil
		return nil
	}
	*m = data
	return nil
}

type taskRow struct {
	ID                int64          `db:"id"`
	UserID            int64          `db:"user_id"`
	SkillID           string         `db:"skill_id"`
	StatusState       string         `db:"status_state"`
	StatusMessage     string         `db:"status_message"`
	StatusUpdatedAt   time.Time      `db:"status_updated_at"`
	StatusCompletedAt *time.Time     `db:"status_completed_at"`
	ContextID         *string        `db:"context_id"`
	History           nullRawMessage `db:"history"`
	Artifacts         nullRawMessage `db:"artifacts"`
	Metadata          nullRawMessage `db:"metadata"`
	CreatedAt         time.Time      `db:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at"`
}

func (r *taskRow) toDomain() (*domain.Task, error) {
	var history []domain.Message
	if len(r.History) > 0 {
		if err := json.Unmarshal(r.History, &history); err != nil {
			return nil, err
		}
	}

	var artifacts []domain.Artifact
	if len(r.Artifacts) > 0 {
		if err := json.Unmarshal(r.Artifacts, &artifacts); err != nil {
			return nil, err
		}
	}

	var metadata map[string]any
	if len(r.Metadata) > 0 {
		if err := json.Unmarshal(r.Metadata, &metadata); err != nil {
			return nil, err
		}
	}

	contextID := ""
	if r.ContextID != nil {
		contextID = *r.ContextID
	}

	task := &domain.Task{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		Status: domain.TaskStatus{
			State:       domain.TaskState(r.StatusState),
			Message:     r.StatusMessage,
			UpdatedAt:   r.StatusUpdatedAt,
			CompletedAt: r.StatusCompletedAt,
		},
		History:   history,
		Artifacts: artifacts,
		ContextID: contextID,
		Metadata:  metadata,
		UserID:    kernel.ID(r.UserID),
		SkillID:   r.SkillID,
	}

	return task, nil
}

func fromDomain(task *domain.Task) (*taskRow, error) {
	history, err := json.Marshal(task.History)
	if err != nil {
		return nil, err
	}

	artifacts, err := json.Marshal(task.Artifacts)
	if err != nil {
		return nil, err
	}

	var metadata nullRawMessage
	if task.Metadata != nil {
		data, err := json.Marshal(task.Metadata)
		if err != nil {
			return nil, err
		}
		metadata = data
	}

	var contextID *string
	if task.ContextID != "" {
		contextID = &task.ContextID
	}

	return &taskRow{
		ID:                task.ID.Int64(),
		UserID:            task.UserID.Int64(),
		SkillID:           task.SkillID,
		StatusState:       string(task.Status.State),
		StatusMessage:     task.Status.Message,
		StatusUpdatedAt:   task.Status.UpdatedAt,
		StatusCompletedAt: task.Status.CompletedAt,
		ContextID:         contextID,
		History:           history,
		Artifacts:         artifacts,
		Metadata:          metadata,
		CreatedAt:         task.CreatedAt,
		UpdatedAt:         task.UpdatedAt,
	}, nil
}

type PostgresTaskRepository struct {
	db *sqlx.DB
}

func NewPostgresTaskRepository(db *sqlx.DB) *PostgresTaskRepository {
	return &PostgresTaskRepository{db: db}
}

func (r *PostgresTaskRepository) Save(ctx context.Context, task *domain.Task) error {
	row, err := fromDomain(task)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO a2a_tasks (id, user_id, skill_id, status_state, status_message, status_updated_at, status_completed_at, context_id, history, artifacts, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO UPDATE SET
			skill_id = EXCLUDED.skill_id,
			status_state = EXCLUDED.status_state,
			status_message = EXCLUDED.status_message,
			status_updated_at = EXCLUDED.status_updated_at,
			status_completed_at = EXCLUDED.status_completed_at,
			context_id = EXCLUDED.context_id,
			history = EXCLUDED.history,
			artifacts = EXCLUDED.artifacts,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`, row.ID, row.UserID, row.SkillID, row.StatusState, row.StatusMessage, row.StatusUpdatedAt, row.StatusCompletedAt, row.ContextID, row.History, row.Artifacts, row.Metadata, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save a2a task", err)
	}
	return nil
}

func (r *PostgresTaskRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Task, error) {
	var row taskRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM a2a_tasks WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find a2a task by id", err)
	}

	return row.toDomain()
}

func (r *PostgresTaskRepository) FindByContextID(ctx context.Context, contextID string, userID kernel.ID) ([]*domain.Task, error) {
	var rows []taskRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM a2a_tasks WHERE context_id = $1 AND user_id = $2 ORDER BY created_at DESC`, contextID, userID.Int64())
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find a2a tasks by context", err)
	}

	tasks := make([]*domain.Task, 0, len(rows))
	for _, row := range rows {
		t, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (r *PostgresTaskRepository) List(ctx context.Context, userID kernel.ID, skillID string, states []domain.TaskState, pageToken string, pageSize int) ([]*domain.Task, string, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	args := []any{userID.Int64()}
	argIdx := 2
	query := `SELECT * FROM a2a_tasks WHERE user_id = $1`

	if skillID != "" {
		query += fmt.Sprintf(" AND skill_id = $%d", argIdx)
		args = append(args, skillID)
		argIdx++
	}

	if len(states) > 0 {
		stateStrs := make([]string, len(states))
		for i, s := range states {
			stateStrs[i] = string(s)
		}
		stateBytes, _ := json.Marshal(stateStrs)
		query += fmt.Sprintf(" AND status_state = ANY($%d::text[])", argIdx)
		args = append(args, string(stateBytes))
		argIdx++
	}

	if pageToken != "" {
		var lastUpdated time.Time
		if err := json.Unmarshal([]byte(`"`+pageToken+`"`), &lastUpdated); err == nil {
			query += fmt.Sprintf(" AND updated_at < $%d", argIdx)
			args = append(args, lastUpdated)
			argIdx++
		}
	}

	query += " ORDER BY updated_at DESC"
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, pageSize)

	var rows []taskRow
	err := r.db.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, "", kernel.NewDomainErrorWithCause(kernel.ErrInternal, "list a2a tasks", err)
	}

	tasks := make([]*domain.Task, 0, len(rows))
	for _, row := range rows {
		t, err := row.toDomain()
		if err != nil {
			return nil, "", err
		}
		tasks = append(tasks, t)
	}

	nextToken := ""
	if len(rows) == pageSize {
		last := rows[len(rows)-1]
		nextToken = last.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}

	return tasks, nextToken, nil
}

func (r *PostgresTaskRepository) Delete(ctx context.Context, id kernel.ID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM a2a_tasks WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete a2a task", err)
	}
	return nil
}
