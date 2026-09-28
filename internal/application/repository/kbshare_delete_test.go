package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openKBShareDeleteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE kb_shares (
			id TEXT PRIMARY KEY,
			knowledge_base_id TEXT NOT NULL,
			deleted_at DATETIME NULL
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE kb_user_shares (
			id TEXT PRIMARY KEY,
			knowledge_base_id TEXT NOT NULL,
			deleted_at DATETIME NULL
		)
	`).Error)

	return db
}

func activeShareCount(t *testing.T, db *gorm.DB, table string, kbID string) int64 {
	t.Helper()

	var count int64
	require.NoError(t, db.Table(table).
		Where("knowledge_base_id = ? AND deleted_at IS NULL", kbID).
		Count(&count).Error)
	return count
}

func deletedShareCount(t *testing.T, db *gorm.DB, table string, kbID string) int64 {
	t.Helper()

	var count int64
	require.NoError(t, db.Table(table).
		Where("knowledge_base_id = ? AND deleted_at IS NOT NULL", kbID).
		Count(&count).Error)
	return count
}

func TestKBShareRepositoryDeleteByKnowledgeBaseIDRevokesOrganizationAndUserShares(t *testing.T) {
	db := openKBShareDeleteTestDB(t)
	repo := NewKBShareRepository(db)

	require.NoError(t, db.Exec(
		"INSERT INTO kb_shares (id, knowledge_base_id) VALUES (?, ?), (?, ?)",
		"org-target", "kb-target",
		"org-other", "kb-other",
	).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO kb_user_shares (id, knowledge_base_id) VALUES (?, ?), (?, ?)",
		"user-target", "kb-target",
		"user-other", "kb-other",
	).Error)

	require.NoError(t, repo.DeleteByKnowledgeBaseID(context.Background(), "kb-target"))

	require.Zero(t, activeShareCount(t, db, "kb_shares", "kb-target"))
	require.Zero(t, activeShareCount(t, db, "kb_user_shares", "kb-target"))
	require.EqualValues(t, 1, deletedShareCount(t, db, "kb_shares", "kb-target"))
	require.EqualValues(t, 1, deletedShareCount(t, db, "kb_user_shares", "kb-target"))

	require.EqualValues(t, 1, activeShareCount(t, db, "kb_shares", "kb-other"))
	require.EqualValues(t, 1, activeShareCount(t, db, "kb_user_shares", "kb-other"))
}

func TestKBShareRepositoryDeleteByKnowledgeBaseIDRollsBackWhenUserShareCleanupFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE kb_shares (
			id TEXT PRIMARY KEY,
			knowledge_base_id TEXT NOT NULL,
			deleted_at DATETIME NULL
		)
	`).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO kb_shares (id, knowledge_base_id) VALUES (?, ?)",
		"org-target", "kb-target",
	).Error)

	repo := NewKBShareRepository(db)
	err = repo.DeleteByKnowledgeBaseID(context.Background(), "kb-target")
	require.Error(t, err, "missing kb_user_shares table must fail the transaction")

	require.EqualValues(t, 1, activeShareCount(t, db, "kb_shares", "kb-target"),
		"organization-share deletion must roll back when direct-user cleanup fails")
	require.Zero(t, deletedShareCount(t, db, "kb_shares", "kb-target"))
}
