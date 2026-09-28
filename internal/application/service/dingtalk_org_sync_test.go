package service

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestBuildDingTalkOrganizationTargetsAggregatesDepartmentsAndStrongestRole(t *testing.T) {
	mappings := []config.DingTalkDepartmentOrganizationMapping{
		{DepartmentID: 10, OrganizationID: "org-a", Role: types.OrgRoleViewer},
		{DepartmentID: 20, OrganizationID: "org-a", Role: types.OrgRoleEditor},
		{DepartmentID: 30, OrganizationID: "org-b", Role: types.OrgRoleViewer},
		{DepartmentID: 40, OrganizationID: "org-unmatched", Role: types.OrgRoleEditor},
	}

	got := buildDingTalkOrganizationTargets(mappings, []int64{30, 20, 10, 20})
	require.Equal(t, []types.DingTalkOrganizationMembershipTarget{
		{
			OrganizationID: "org-a",
			Role:           types.OrgRoleEditor,
			DepartmentIDs:  []int64{10, 20},
		},
		{
			OrganizationID: "org-b",
			Role:           types.OrgRoleViewer,
			DepartmentIDs:  []int64{30},
		},
	}, got)
}

func TestBuildDingTalkOrganizationTargetsReturnsEmptyForNoMappedDepartments(t *testing.T) {
	got := buildDingTalkOrganizationTargets(
		[]config.DingTalkDepartmentOrganizationMapping{{
			DepartmentID: 10, OrganizationID: "org-a", Role: types.OrgRoleViewer,
		}},
		[]int64{99},
	)
	require.Empty(t, got)
}
