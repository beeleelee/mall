package a2a

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	domain "github.com/beeleelee/mall/domain/a2a"
	"github.com/beeleelee/mall/domain/kernel"
)

type integrationFixture struct {
	taskRepo *PostgresTaskRepository
	pushRepo *PostgresPushNotificationConfigRepository
	db       *sqlx.DB
	schema   string
	cleanup  func()
}

func servicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
	if err != nil {
		return false
	}
	pg.Close()
	return true
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()

	if !servicesUp() {
		t.Skip("integration: need 'docker compose up postgres' running")
	}

	dsn := "postgres://mall:mall_dev@localhost:5432/mall?sslmode=disable"
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}

	schema := fmt.Sprintf("test_a2a_%d", rand.Int63())
	if _, err := db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(fmt.Sprintf(`SET search_path TO "%s"`, schema)); err != nil {
		t.Fatal(err)
	}

	upSQL := `
	CREATE TABLE IF NOT EXISTS a2a_tasks (
		id BIGINT PRIMARY KEY,
		user_id BIGINT NOT NULL,
		skill_id VARCHAR(255) NOT NULL DEFAULT '',
		status_state VARCHAR(50) NOT NULL DEFAULT 'submitted',
		status_message TEXT NOT NULL DEFAULT '',
		status_updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		status_completed_at TIMESTAMP WITH TIME ZONE,
		context_id VARCHAR(255),
		history JSONB NOT NULL DEFAULT '[]'::jsonb,
		artifacts JSONB NOT NULL DEFAULT '[]'::jsonb,
		metadata JSONB,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS a2a_push_notification_configs (
		id BIGINT PRIMARY KEY,
		task_id BIGINT NOT NULL REFERENCES a2a_tasks(id) ON DELETE CASCADE,
		url TEXT NOT NULL,
		auth_scheme VARCHAR(50) NOT NULL DEFAULT '',
		auth_credentials TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);`
	if _, err := db.Exec(upSQL); err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
		db.Close()
	}

	return &integrationFixture{
		taskRepo: NewPostgresTaskRepository(db),
		pushRepo: NewPostgresPushNotificationConfigRepository(db),
		db:       db,
		schema:   schema,
		cleanup:  cleanup,
	}
}

func TestIntegrationTaskSaveAndFindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	task := domain.NewTask(1, 100, "catalog", "ctx-1")
	task.AddMessage(domain.Message{
		Role:  domain.RoleUser,
		Parts: []domain.Part{domain.TextPart("search for shoes")},
	})

	if err := f.taskRepo.Save(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	found, err := f.taskRepo.FindByID(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("expected task to be found")
	}
	if found.ID != 1 {
		t.Errorf("expected id 1, got %d", found.ID)
	}
	if found.UserID != 100 {
		t.Errorf("expected user 100, got %d", found.UserID)
	}
	if found.SkillID != "catalog" {
		t.Errorf("expected skill catalog, got %s", found.SkillID)
	}
	if found.ContextID != "ctx-1" {
		t.Errorf("expected context ctx-1, got %s", found.ContextID)
	}
	if found.Status.State != domain.TaskStateSubmitted {
		t.Errorf("expected submitted, got %s", found.Status.State)
	}
}

func TestIntegrationTaskNotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	found, err := f.taskRepo.FindByID(context.Background(), 999)
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Error("expected nil for non-existent task")
	}
}

func TestIntegrationTaskUpdate(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	task := domain.NewTask(2, 100, "checkout", "")
	task.Transition(domain.TaskStateWorking, "processing")
	if err := f.taskRepo.Save(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	task.Transition(domain.TaskStateCompleted, "done")
	task.AddArtifact(domain.Artifact{
		ID:    "art-1",
		Name:  "result",
		Parts: []domain.Part{domain.TextPart("completed")},
	})
	if err := f.taskRepo.Save(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	found, err := f.taskRepo.FindByID(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if found.Status.State != domain.TaskStateCompleted {
		t.Errorf("expected completed, got %s", found.Status.State)
	}
	if len(found.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(found.Artifacts))
	}
	if found.Artifacts[0].Name != "result" {
		t.Errorf("expected artifact name result, got %s", found.Artifacts[0].Name)
	}
}

func TestIntegrationTaskList(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	for i := int64(10); i < 15; i++ {
		task := domain.NewTask(kernel.ID(i), 200, "catalog", "")
		if err := f.taskRepo.Save(context.Background(), task); err != nil {
			t.Fatal(err)
		}
	}

	tasks, next, err := f.taskRepo.List(context.Background(), 200, "", nil, "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
	if next == "" {
		t.Error("expected next page token")
	}

	tasks2, next2, err := f.taskRepo.List(context.Background(), 200, "", nil, next, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks2) != 2 {
		t.Errorf("expected 2 more tasks, got %d", len(tasks2))
	}
	if next2 != "" {
		t.Errorf("expected empty next token, got %s", next2)
	}
}

func TestIntegrationTaskListBySkill(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	task1 := domain.NewTask(20, 300, "catalog", "")
	f.taskRepo.Save(context.Background(), task1)
	task2 := domain.NewTask(21, 300, "cart", "")
	f.taskRepo.Save(context.Background(), task2)

	tasks, _, err := f.taskRepo.List(context.Background(), 300, "catalog", nil, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 catalog task, got %d", len(tasks))
	}
}

func TestIntegrationTaskHistoryPersistence(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	task := domain.NewTask(30, 400, "order", "")
	task.AddMessage(domain.Message{
		Role:  domain.RoleUser,
		Parts: []domain.Part{domain.TextPart("show my orders")},
	})
	task.AddMessage(domain.Message{
		Role:  domain.RoleAgent,
		Parts: []domain.Part{domain.TextPart("here are your orders")},
	})
	if err := f.taskRepo.Save(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	found, err := f.taskRepo.FindByID(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(found.History) != 2 {
		t.Errorf("expected 2 messages, got %d", len(found.History))
	}
	if found.History[0].Role != domain.RoleUser {
		t.Errorf("expected first message role user, got %s", found.History[0].Role)
	}
}

func TestIntegrationTaskDelete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	task := domain.NewTask(40, 500, "catalog", "")
	if err := f.taskRepo.Save(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	if err := f.taskRepo.Delete(context.Background(), 40); err != nil {
		t.Fatal(err)
	}

	found, err := f.taskRepo.FindByID(context.Background(), 40)
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Error("expected nil after delete")
	}
}

func TestIntegrationPushConfigSaveAndFind(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	task := domain.NewTask(50, 600, "checkout", "")
	if err := f.taskRepo.Save(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	cfg := domain.NewPushNotificationConfig(60, 50, "https://example.com/hook", domain.AuthInfo{
		Scheme: "bearer", Credentials: "tok-123",
	})
	if err := f.pushRepo.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	found, err := f.pushRepo.FindByID(context.Background(), 60)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("expected config to be found")
	}
	if found.URL != "https://example.com/hook" {
		t.Errorf("expected url, got %s", found.URL)
	}
	if found.AuthInfo.Scheme != "bearer" {
		t.Errorf("expected bearer auth, got %s", found.AuthInfo.Scheme)
	}
}

func TestIntegrationPushConfigFindByTask(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	task := domain.NewTask(70, 700, "order", "")
	f.taskRepo.Save(context.Background(), task)

	cfg1 := domain.NewPushNotificationConfig(71, 70, "https://hook1.example.com", domain.AuthInfo{})
	cfg2 := domain.NewPushNotificationConfig(72, 70, "https://hook2.example.com", domain.AuthInfo{})
	f.pushRepo.Save(context.Background(), cfg1)
	f.pushRepo.Save(context.Background(), cfg2)

	configs, err := f.pushRepo.FindByTaskID(context.Background(), 70)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 2 {
		t.Errorf("expected 2 configs, got %d", len(configs))
	}
}

func TestIntegrationPushConfigDelete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	task := domain.NewTask(80, 800, "catalog", "")
	f.taskRepo.Save(context.Background(), task)

	cfg := domain.NewPushNotificationConfig(81, 80, "https://example.com/hook", domain.AuthInfo{})
	f.pushRepo.Save(context.Background(), cfg)

	if err := f.pushRepo.Delete(context.Background(), 81); err != nil {
		t.Fatal(err)
	}

	found, err := f.pushRepo.FindByID(context.Background(), 81)
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Error("expected nil after delete")
	}
}

func TestIntegrationPushConfigCascadeDelete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	task := domain.NewTask(90, 900, "catalog", "")
	f.taskRepo.Save(context.Background(), task)
	domain.NewPushNotificationConfig(91, 90, "https://example.com/hook", domain.AuthInfo{})

	// Delete the task - push config should cascade
	if err := f.taskRepo.Delete(context.Background(), 90); err != nil {
		t.Fatal(err)
	}

	configs, err := f.pushRepo.FindByTaskID(context.Background(), 90)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 configs after task delete, got %d", len(configs))
	}
}
