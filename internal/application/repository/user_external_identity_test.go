package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExternalIdentityRepositoryBindsStableSubject(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:external_identity?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&types.Tenant{}, &types.User{}, &types.ExternalIdentity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userRepo := NewUserRepository(db)
	externalRepo, ok := userRepo.(interfaces.ExternalIdentityRepository)
	if !ok {
		t.Fatal("user repository does not implement ExternalIdentityRepository")
	}

	first := &types.User{
		ID:           "user-1",
		Username:     "ding-one",
		Email:        "one@example.com",
		PasswordHash: "hashed",
		IsActive:     true,
	}
	second := &types.User{
		ID:           "user-2",
		Username:     "ding-two",
		Email:        "two@example.com",
		PasswordHash: "hashed",
		IsActive:     true,
	}
	if err := userRepo.CreateUser(context.Background(), first); err != nil {
		t.Fatalf("create first user: %v", err)
	}
	if err := userRepo.CreateUser(context.Background(), second); err != nil {
		t.Fatalf("create second user: %v", err)
	}

	identity := &types.ExternalIdentity{
		Provider: types.ExternalIdentityProviderDingTalk,
		Subject:  "union-1",
		UserID:   first.ID,
	}
	if err := externalRepo.BindExternalIdentity(context.Background(), identity); err != nil {
		t.Fatalf("bind identity: %v", err)
	}

	got, err := externalRepo.GetUserByExternalIdentity(
		context.Background(),
		types.ExternalIdentityProviderDingTalk,
		"union-1",
	)
	if err != nil {
		t.Fatalf("lookup identity: %v", err)
	}
	if got.ID != first.ID {
		t.Fatalf("resolved user = %q, want %q", got.ID, first.ID)
	}

	rebind := &types.ExternalIdentity{
		Provider: types.ExternalIdentityProviderDingTalk,
		Subject:  "union-1",
		UserID:   second.ID,
	}
	if err := externalRepo.BindExternalIdentity(context.Background(), rebind); err == nil {
		t.Fatal("re-binding the same provider+subject unexpectedly succeeded")
	}

	got, err = externalRepo.GetUserByExternalIdentity(
		context.Background(),
		types.ExternalIdentityProviderDingTalk,
		"union-1",
	)
	if err != nil || got.ID != first.ID {
		t.Fatalf("binding changed after rejected rebind: user=%#v err=%v", got, err)
	}
}
