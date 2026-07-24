package identity

import (
	"testing"
	"time"

	domain "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
)

func seedUser(t *testing.T, f *integrationFixture, id int64, email string) {
	t.Helper()
	u, err := domain.NewUser(kernel.ID(id), email, "Test User", testPassword(), nil)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	if err := f.repo.Save(ctx, u); err != nil {
		t.Fatalf("save user: %v", err)
	}
}

func TestPasswordResetToken_SaveAndFindByHash(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	seedUser(t, f, 100, "alice@example.com")

	token := domain.NewPasswordResetToken(1, 100, "abc123hash", time.Now().Add(1*time.Hour))
	if err := f.tokenRepo.Save(ctx, token); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := f.tokenRepo.FindByHash(ctx, "abc123hash")
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if found.ID != 1 {
		t.Errorf("ID = %d, want 1", found.ID)
	}
	if found.UserID != 100 {
		t.Errorf("UserID = %d, want 100", found.UserID)
	}
	if found.TokenHash != "abc123hash" {
		t.Errorf("TokenHash = %q", found.TokenHash)
	}
	if found.Used {
		t.Error("expected token to be unused")
	}
}

func TestPasswordResetToken_FindByHash_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.tokenRepo.FindByHash(ctx, "nonexistent")
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestPasswordResetToken_SaveAndMarkUsed(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	seedUser(t, f, 100, "bob@example.com")

	token := domain.NewPasswordResetToken(1, 100, "markusedhash", time.Now().Add(1*time.Hour))
	if err := f.tokenRepo.Save(ctx, token); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := f.tokenRepo.MarkUsed(ctx, 1); err != nil {
		t.Fatalf("MarkUsed: %v", err)
	}

	found, err := f.tokenRepo.FindByHash(ctx, "markusedhash")
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if !found.Used {
		t.Error("expected token to be marked as used")
	}
}

func TestPasswordResetToken_MarkUsed_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	err := f.tokenRepo.MarkUsed(ctx, 999)
	if err != nil {
		t.Logf("MarkUsed on non-existent ID returned error: %v (acceptable)", err)
	}
}

func TestPasswordResetToken_DeleteExpired(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	seedUser(t, f, 100, "carol@example.com")
	seedUser(t, f, 101, "dave@example.com")

	expired := domain.NewPasswordResetToken(1, 100, "expiredhash", time.Now().Add(-1*time.Hour))
	valid := domain.NewPasswordResetToken(2, 101, "validhash", time.Now().Add(1*time.Hour))
	if err := f.tokenRepo.Save(ctx, expired); err != nil {
		t.Fatalf("Save expired: %v", err)
	}
	if err := f.tokenRepo.Save(ctx, valid); err != nil {
		t.Fatalf("Save valid: %v", err)
	}

	if err := f.tokenRepo.DeleteExpired(ctx); err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}

	_, err := f.tokenRepo.FindByHash(ctx, "expiredhash")
	if !kernel.IsNotFound(err) {
		t.Errorf("expected expired token to be deleted, got %v", err)
	}

	found, err := f.tokenRepo.FindByHash(ctx, "validhash")
	if err != nil {
		t.Fatalf("expected valid token to remain, got %v", err)
	}
	if found.ID != 2 {
		t.Errorf("valid token ID = %d, want 2", found.ID)
	}
}

func TestPasswordResetToken_DeleteExpired_NoExpired(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	seedUser(t, f, 100, "eve@example.com")

	token := domain.NewPasswordResetToken(1, 100, "hash", time.Now().Add(1*time.Hour))
	if err := f.tokenRepo.Save(ctx, token); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := f.tokenRepo.DeleteExpired(ctx); err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}

	_, err := f.tokenRepo.FindByHash(ctx, "hash")
	if err != nil {
		t.Errorf("expected token to remain, got %v", err)
	}
}

func TestPasswordResetToken_UpsertIdempotent(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	seedUser(t, f, 100, "frank@example.com")

	token := domain.NewPasswordResetToken(1, 100, "hash", time.Now().Add(1*time.Hour))
	if err := f.tokenRepo.Save(ctx, token); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	token.MarkUsed()
	if err := f.tokenRepo.Save(ctx, token); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	found, err := f.tokenRepo.FindByHash(ctx, "hash")
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if !found.Used {
		t.Error("expected token to be used after MarkUsed + Save")
	}
}

func TestPasswordResetToken_UpsertSameData(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	seedUser(t, f, 100, "grace@example.com")

	token := domain.NewPasswordResetToken(1, 100, "samehash", time.Now().Add(1*time.Hour))
	if err := f.tokenRepo.Save(ctx, token); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	if err := f.tokenRepo.Save(ctx, token); err != nil {
		t.Fatalf("second Save (same data): %v", err)
	}
}
