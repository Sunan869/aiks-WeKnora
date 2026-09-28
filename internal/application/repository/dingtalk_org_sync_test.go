package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openDingTalkOrganizationSyncDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.Organization{},
		&types.OrganizationTenantMember{},
		&types.DingTalkManagedOrganizationMembership{},
	))
	return db
}

func seedSyncOrganization(t *testing.T, db *gorm.DB, id string, limit int) {
	t.Helper()
	require.NoError(t, db.Create(&types.Organization{
		ID: id, Name: id, OwnerID: "owner", OwnerTenantID: 999, MemberLimit: limit,
	}).Error)
}

func TestSyncDingTalkOrganizationMembershipsCreatesUpdatesAndRevokesManagedMembership(t *testing.T) {
	db := openDingTalkOrganizationSyncDB(t)
	seedSyncOrganization(t, db, "org-a", 50)
	repo := &userRepository{db: db}

	err := repo.SyncDingTalkOrganizationMemberships(
		context.Background(), "user-1", 10, "corp-a",
		[]types.DingTalkOrganizationMembershipTarget{{
			OrganizationID: "org-a",
			Role:           types.OrgRoleViewer,
			DepartmentIDs:  []int64{8, 3, 8},
		}},
	)
	require.NoError(t, err)

	var member types.OrganizationTenantMember
	require.NoError(t, db.Where("organization_id = ? AND tenant_id = ?", "org-a", 10).First(&member).Error)
	require.Equal(t, types.OrgRoleViewer, member.Role)
	require.Equal(t, "user-1", member.RepresentativeUserID)

	var managed types.DingTalkManagedOrganizationMembership
	require.NoError(t, db.Where("user_id = ? AND tenant_id = ? AND organization_id = ?", "user-1", 10, "org-a").First(&managed).Error)
	require.Equal(t, "[3,8]", managed.DepartmentIDs)

	err = repo.SyncDingTalkOrganizationMemberships(
		context.Background(), "user-1", 10, "corp-a",
		[]types.DingTalkOrganizationMembershipTarget{{
			OrganizationID: "org-a",
			Role:           types.OrgRoleEditor,
			DepartmentIDs:  []int64{3},
		}},
	)
	require.NoError(t, err)
	require.NoError(t, db.Where("organization_id = ? AND tenant_id = ?", "org-a", 10).First(&member).Error)
	require.Equal(t, types.OrgRoleEditor, member.Role)

	require.NoError(t, repo.SyncDingTalkOrganizationMemberships(
		context.Background(), "user-1", 10, "corp-a", nil,
	))
	require.ErrorIs(t,
		db.Where("organization_id = ? AND tenant_id = ?", "org-a", 10).First(&member).Error,
		gorm.ErrRecordNotFound,
	)
	require.ErrorIs(t,
		db.Where("user_id = ? AND tenant_id = ? AND organization_id = ?", "user-1", 10, "org-a").First(&managed).Error,
		gorm.ErrRecordNotFound,
	)
}

func TestSyncDingTalkOrganizationMembershipsDoesNotClaimManualMembership(t *testing.T) {
	db := openDingTalkOrganizationSyncDB(t)
	seedSyncOrganization(t, db, "org-manual", 50)
	repo := &userRepository{db: db}
	require.NoError(t, db.Create(&types.OrganizationTenantMember{
		ID: "manual-member", OrganizationID: "org-manual", TenantID: 10,
		Role: types.OrgRoleAdmin, RepresentativeUserID: "admin-user",
	}).Error)

	require.NoError(t, repo.SyncDingTalkOrganizationMemberships(
		context.Background(), "user-1", 10, "corp-a",
		[]types.DingTalkOrganizationMembershipTarget{{
			OrganizationID: "org-manual",
			Role:           types.OrgRoleViewer,
			DepartmentIDs:  []int64{1},
		}},
	))

	var member types.OrganizationTenantMember
	require.NoError(t, db.Where("id = ?", "manual-member").First(&member).Error)
	require.Equal(t, types.OrgRoleAdmin, member.Role)

	var count int64
	require.NoError(t, db.Model(&types.DingTalkManagedOrganizationMembership{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestSyncDingTalkOrganizationMembershipsAggregatesManagedUsersForSameTenant(t *testing.T) {
	db := openDingTalkOrganizationSyncDB(t)
	seedSyncOrganization(t, db, "org-shared", 50)
	repo := &userRepository{db: db}

	require.NoError(t, repo.SyncDingTalkOrganizationMemberships(
		context.Background(), "user-viewer", 10, "corp-a",
		[]types.DingTalkOrganizationMembershipTarget{{
			OrganizationID: "org-shared", Role: types.OrgRoleViewer, DepartmentIDs: []int64{1},
		}},
	))
	require.NoError(t, repo.SyncDingTalkOrganizationMemberships(
		context.Background(), "user-editor", 10, "corp-a",
		[]types.DingTalkOrganizationMembershipTarget{{
			OrganizationID: "org-shared", Role: types.OrgRoleEditor, DepartmentIDs: []int64{2},
		}},
	))

	var member types.OrganizationTenantMember
	require.NoError(t, db.Where("organization_id = ? AND tenant_id = ?", "org-shared", 10).First(&member).Error)
	require.Equal(t, types.OrgRoleEditor, member.Role)

	require.NoError(t, repo.SyncDingTalkOrganizationMemberships(
		context.Background(), "user-editor", 10, "corp-a", nil,
	))
	require.NoError(t, db.Where("organization_id = ? AND tenant_id = ?", "org-shared", 10).First(&member).Error)
	require.Equal(t, types.OrgRoleViewer, member.Role)

	require.NoError(t, repo.SyncDingTalkOrganizationMemberships(
		context.Background(), "user-viewer", 10, "corp-a", nil,
	))
	require.ErrorIs(t,
		db.Where("organization_id = ? AND tenant_id = ?", "org-shared", 10).First(&member).Error,
		gorm.ErrRecordNotFound,
	)
}
