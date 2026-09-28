package repository

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ interfaces.DingTalkOrganizationSyncRepository = (*userRepository)(nil)

func normalizeDingTalkMembershipTargets(
	desired []types.DingTalkOrganizationMembershipTarget,
) []types.DingTalkOrganizationMembershipTarget {
	type aggregate struct {
		role types.OrgMemberRole
		deps map[int64]struct{}
	}
	byOrg := make(map[string]*aggregate)
	for _, target := range desired {
		orgID := strings.TrimSpace(target.OrganizationID)
		if orgID == "" || !target.Role.IsValid() || target.Role == types.OrgRoleAdmin {
			continue
		}
		item := byOrg[orgID]
		if item == nil {
			item = &aggregate{role: target.Role, deps: map[int64]struct{}{}}
			byOrg[orgID] = item
		} else if target.Role.HasPermission(item.role) {
			item.role = target.Role
		}
		for _, depID := range target.DepartmentIDs {
			if depID > 0 {
				item.deps[depID] = struct{}{}
			}
		}
	}

	orgIDs := make([]string, 0, len(byOrg))
	for orgID := range byOrg {
		orgIDs = append(orgIDs, orgID)
	}
	sort.Strings(orgIDs)

	result := make([]types.DingTalkOrganizationMembershipTarget, 0, len(orgIDs))
	for _, orgID := range orgIDs {
		item := byOrg[orgID]
		deps := make([]int64, 0, len(item.deps))
		for depID := range item.deps {
			deps = append(deps, depID)
		}
		sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })
		result = append(result, types.DingTalkOrganizationMembershipTarget{
			OrganizationID: orgID,
			Role:           item.role,
			DepartmentIDs:  deps,
		})
	}
	return result
}

func marshalDepartmentIDs(ids []int64) (string, error) {
	if ids == nil {
		ids = []int64{}
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SyncDingTalkOrganizationMemberships reconciles the organization memberships
// that are owned by DingTalk department mapping for one local user.
//
// Existing organization_tenant_members rows are never claimed when no managed
// marker exists for the same tenant/org. This protects memberships created by
// invitations or administrators. Once an org has at least one managed marker,
// all DingTalk users in the same tenant may contribute desired access to it and
// the tenant-level role is derived from the strongest remaining managed grant.
func (r *userRepository) SyncDingTalkOrganizationMemberships(
	ctx context.Context,
	userID string,
	tenantID uint64,
	corpID string,
	desired []types.DingTalkOrganizationMembershipTarget,
) error {
	userID = strings.TrimSpace(userID)
	corpID = strings.TrimSpace(corpID)
	if userID == "" {
		return errors.New("DingTalk organization sync requires user id")
	}
	if tenantID == 0 {
		return errors.New("DingTalk organization sync requires tenant id")
	}
	if corpID == "" {
		return errors.New("DingTalk organization sync requires corp id")
	}

	desired = normalizeDingTalkMembershipTargets(desired)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current []*types.DingTalkManagedOrganizationMembership
		if err := tx.
			Where("user_id = ? AND tenant_id = ?", userID, tenantID).
			Find(&current).Error; err != nil {
			return err
		}
		currentByOrg := make(map[string]*types.DingTalkManagedOrganizationMembership, len(current))
		for _, row := range current {
			if row != nil {
				currentByOrg[row.OrganizationID] = row
			}
		}

		desiredByOrg := make(map[string]types.DingTalkOrganizationMembershipTarget, len(desired))
		for _, target := range desired {
			desiredByOrg[target.OrganizationID] = target
			if err := r.applyDingTalkMembershipTarget(
				tx, userID, tenantID, corpID, target, currentByOrg[target.OrganizationID],
			); err != nil {
				return err
			}
		}

		for _, stale := range current {
			if stale == nil {
				continue
			}
			if _, keep := desiredByOrg[stale.OrganizationID]; keep {
				continue
			}
			if err := tx.Delete(&types.DingTalkManagedOrganizationMembership{}, "id = ?", stale.ID).Error; err != nil {
				return err
			}
			if err := reconcileManagedTenantOrganizationRole(tx, tenantID, stale.OrganizationID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *userRepository) applyDingTalkMembershipTarget(
	tx *gorm.DB,
	userID string,
	tenantID uint64,
	corpID string,
	target types.DingTalkOrganizationMembershipTarget,
	existingManaged *types.DingTalkManagedOrganizationMembership,
) error {
	var org types.Organization
	q := tx
	if tx.Name() != "sqlite" {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.Where("id = ?", target.OrganizationID).First(&org).Error; err != nil {
		return err
	}

	var member types.OrganizationTenantMember
	memberErr := tx.
		Where("organization_id = ? AND tenant_id = ?", target.OrganizationID, tenantID).
		First(&member).Error
	if memberErr != nil && !errors.Is(memberErr, gorm.ErrRecordNotFound) {
		return memberErr
	}

	// If the tenant is already a member but no managed record exists for this
	// tenant/org, that relationship is human-managed. Do not claim or mutate it.
	if existingManaged == nil && memberErr == nil {
		var managedCount int64
		if err := tx.Model(&types.DingTalkManagedOrganizationMembership{}).
			Where("tenant_id = ? AND organization_id = ?", tenantID, target.OrganizationID).
			Count(&managedCount).Error; err != nil {
			return err
		}
		if managedCount == 0 {
			return nil
		}
	}

	if errors.Is(memberErr, gorm.ErrRecordNotFound) {
		if org.MemberLimit > 0 {
			var memberCount int64
			if err := tx.Model(&types.OrganizationTenantMember{}).
				Where("organization_id = ?", target.OrganizationID).
				Count(&memberCount).Error; err != nil {
				return err
			}
			if memberCount >= int64(org.MemberLimit) {
				return ErrOrgMemberLimitReached
			}
		}
		now := time.Now()
		member = types.OrganizationTenantMember{
			ID:                   uuid.New().String(),
			OrganizationID:       target.OrganizationID,
			TenantID:             tenantID,
			Role:                 target.Role,
			RepresentativeUserID: userID,
			JoinedAt:             &now,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
	}

	departmentIDs, err := marshalDepartmentIDs(target.DepartmentIDs)
	if err != nil {
		return err
	}
	now := time.Now()
	managed := &types.DingTalkManagedOrganizationMembership{
		ID:             uuid.New().String(),
		UserID:         userID,
		TenantID:       tenantID,
		OrganizationID: target.OrganizationID,
		CorpID:         corpID,
		DepartmentIDs:  departmentIDs,
		Role:           target.Role,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if existingManaged != nil {
		managed.ID = existingManaged.ID
		managed.CreatedAt = existingManaged.CreatedAt
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "tenant_id"},
			{Name: "organization_id"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"corp_id":        corpID,
			"department_ids": departmentIDs,
			"role":           target.Role,
			"updated_at":     now,
		}),
	}).Create(managed).Error; err != nil {
		return err
	}

	return reconcileManagedTenantOrganizationRole(tx, tenantID, target.OrganizationID)
}

func reconcileManagedTenantOrganizationRole(tx *gorm.DB, tenantID uint64, orgID string) error {
	var managed []*types.DingTalkManagedOrganizationMembership
	if err := tx.
		Where("tenant_id = ? AND organization_id = ?", tenantID, orgID).
		Find(&managed).Error; err != nil {
		return err
	}
	if len(managed) == 0 {
		result := tx.
			Where("organization_id = ? AND tenant_id = ?", orgID, tenantID).
			Delete(&types.OrganizationTenantMember{})
		return result.Error
	}

	role := types.OrgRoleViewer
	for _, row := range managed {
		if row != nil && row.Role.HasPermission(role) {
			role = row.Role
		}
	}
	return tx.Model(&types.OrganizationTenantMember{}).
		Where("organization_id = ? AND tenant_id = ?", orgID, tenantID).
		Updates(map[string]interface{}{
			"role":       role,
			"updated_at": time.Now(),
		}).Error
}
