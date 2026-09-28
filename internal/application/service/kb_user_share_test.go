package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type directShareKBRepo struct {
	interfaces.KnowledgeBaseRepository
	kb *types.KnowledgeBase
}

func (r directShareKBRepo) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return r.kb, nil
}

type directOrgShareRepo struct {
	interfaces.KBShareRepository
}

func (directOrgShareRepo) ListByKnowledgeBase(context.Context, string) ([]*types.KnowledgeBaseShare, error) {
	return nil, nil
}

type directUserShareRepo struct {
	shares map[string]*types.KnowledgeBaseUserShare
}

func newDirectUserShareRepo() *directUserShareRepo {
	return &directUserShareRepo{shares: map[string]*types.KnowledgeBaseUserShare{}}
}

func (r *directUserShareRepo) CreateUserShare(_ context.Context, share *types.KnowledgeBaseUserShare) error {
	key := share.KnowledgeBaseID + ":" + share.TargetUserID
	if _, ok := r.shares[key]; ok {
		return repository.ErrKBUserShareAlreadyExists
	}
	copied := *share
	r.shares[key] = &copied
	return nil
}

func (r *directUserShareRepo) GetUserShareByID(_ context.Context, id string) (*types.KnowledgeBaseUserShare, error) {
	for _, share := range r.shares {
		if share.ID == id {
			copied := *share
			return &copied, nil
		}
	}
	return nil, repository.ErrKBUserShareNotFound
}

func (r *directUserShareRepo) GetUserShareByKBAndUser(_ context.Context, kbID, userID string) (*types.KnowledgeBaseUserShare, error) {
	share := r.shares[kbID+":"+userID]
	if share == nil {
		return nil, repository.ErrKBUserShareNotFound
	}
	copied := *share
	return &copied, nil
}

func (r *directUserShareRepo) UpdateUserShare(_ context.Context, share *types.KnowledgeBaseUserShare) error {
	copied := *share
	r.shares[share.KnowledgeBaseID+":"+share.TargetUserID] = &copied
	return nil
}

func (r *directUserShareRepo) DeleteUserShare(_ context.Context, id string) error {
	for key, share := range r.shares {
		if share.ID == id {
			delete(r.shares, key)
			return nil
		}
	}
	return repository.ErrKBUserShareNotFound
}

func (r *directUserShareRepo) ListUserSharesByKnowledgeBase(_ context.Context, kbID string) ([]*types.KnowledgeBaseUserShare, error) {
	var out []*types.KnowledgeBaseUserShare
	for _, share := range r.shares {
		if share.KnowledgeBaseID == kbID {
			copied := *share
			out = append(out, &copied)
		}
	}
	return out, nil
}

func (r *directUserShareRepo) ListUserSharesForUser(_ context.Context, userID string) ([]*types.KnowledgeBaseUserShare, error) {
	var out []*types.KnowledgeBaseUserShare
	for _, share := range r.shares {
		if share.TargetUserID == userID {
			copied := *share
			copied.KnowledgeBase = &types.KnowledgeBase{ID: share.KnowledgeBaseID, TenantID: share.SourceTenantID}
			out = append(out, &copied)
		}
	}
	return out, nil
}

func directShareContext(userID string, role types.TenantRole) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(20))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	return types.WithCaller(ctx, types.Caller{TenantID: 20, UserID: userID, Role: role})
}

func TestDirectUserShareUpsertsAndRejectsAdmin(t *testing.T) {
	users := newDirectUserShareRepo()
	svc := &kbShareService{
		shareRepo: directOrgShareRepo{},
		userShareRepo: users,
		kbRepo: directShareKBRepo{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 10}},
	}

	share, err := svc.ShareKnowledgeBaseToUser(
		context.Background(), "kb-1", "target", "owner", 10, types.OrgRoleViewer,
	)
	require.NoError(t, err)
	require.Equal(t, types.OrgRoleViewer, share.Permission)

	updated, err := svc.ShareKnowledgeBaseToUser(
		context.Background(), "kb-1", "target", "owner", 10, types.OrgRoleEditor,
	)
	require.NoError(t, err)
	require.Equal(t, share.ID, updated.ID)
	require.Equal(t, types.OrgRoleEditor, updated.Permission)
	require.Len(t, users.shares, 1)

	_, err = svc.ShareKnowledgeBaseToUser(
		context.Background(), "kb-1", "other", "owner", 10, types.OrgRoleAdmin,
	)
	require.ErrorIs(t, err, ErrInvalidRole)
}

func TestDirectUserShareIsIdentityScopedAndViewerCapped(t *testing.T) {
	users := newDirectUserShareRepo()
	users.shares["kb-1:alice"] = &types.KnowledgeBaseUserShare{
		ID: "share-1", KnowledgeBaseID: "kb-1", TargetUserID: "alice",
		SourceTenantID: 10, Permission: types.OrgRoleEditor,
	}
	svc := &kbShareService{
		shareRepo: directOrgShareRepo{},
		userShareRepo: users,
		kbRepo: directShareKBRepo{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 10}},
	}

	role, shared, err := svc.CheckTenantKBPermission(
		directShareContext("alice", types.TenantRoleViewer),
		"kb-1", 20, types.TenantRoleViewer,
	)
	require.NoError(t, err)
	require.True(t, shared)
	require.Equal(t, types.OrgRoleViewer, role)

	role, shared, err = svc.CheckTenantKBPermission(
		directShareContext("bob", types.TenantRoleAdmin),
		"kb-1", 20, types.TenantRoleAdmin,
	)
	require.NoError(t, err)
	require.False(t, shared)
	require.Empty(t, role)
}

func TestDirectUserShareRecipientCannotMutateOwnerGrant(t *testing.T) {
	users := newDirectUserShareRepo()
	users.shares["kb-1:alice"] = &types.KnowledgeBaseUserShare{
		ID: "share-1", KnowledgeBaseID: "kb-1", TargetUserID: "alice",
		SharedByUserID: "owner", SourceTenantID: 10, Permission: types.OrgRoleViewer,
	}
	svc := &kbShareService{
		shareRepo: directOrgShareRepo{},
		userShareRepo: users,
		kbRepo: directShareKBRepo{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 10}},
	}
	ctx := directShareContext("alice", types.TenantRoleAdmin)
	err := svc.UpdateUserSharePermission(ctx, "kb-1", "share-1", types.OrgRoleEditor, "alice", 20)
	require.ErrorIs(t, err, ErrSharePermissionDenied)
}
