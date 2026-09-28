package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const (
	dingTalkAppTokenEndpoint       = "https://api.dingtalk.com/v1.0/oauth2/accessToken"
	dingTalkUserIDByUnionEndpoint  = "https://oapi.dingtalk.com/topapi/user/getbyunionid"
	dingTalkUserDetailEndpoint     = "https://oapi.dingtalk.com/topapi/v2/user/get"
	dingTalkAppTokenRefreshAdvance = 5 * time.Minute
)

type dingTalkAppTokenCacheEntry struct {
	token     string
	expiresAt time.Time
}

var dingTalkAppTokenCache = struct {
	sync.Mutex
	entries map[string]dingTalkAppTokenCacheEntry
}{
	entries: make(map[string]dingTalkAppTokenCacheEntry),
}

type dingTalkAppTokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpireIn    int64  `json:"expireIn"`
}

type dingTalkUserIDByUnionResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Result  struct {
		UserID string `json:"userid"`
	} `json:"result"`
}

type dingTalkUserDetailResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Result  struct {
		UserID     string  `json:"userid"`
		UnionID    string  `json:"unionid"`
		DeptIDList []int64 `json:"dept_id_list"`
	} `json:"result"`
}

func dingTalkAppTokenCacheKey(cfg *config.DingTalkAuthConfig) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.ClientID) + "\x00" + strings.TrimSpace(cfg.CorpID)
}

func (s *userService) fetchDingTalkAppToken(ctx context.Context) (string, error) {
	cfg := s.config.DingTalkAuth
	if cfg == nil {
		return "", errors.New("DingTalk configuration is unavailable")
	}
	key := dingTalkAppTokenCacheKey(cfg)
	if key == "" {
		return "", errors.New("DingTalk client id is required")
	}

	dingTalkAppTokenCache.Lock()
	defer dingTalkAppTokenCache.Unlock()

	if cached, ok := dingTalkAppTokenCache.entries[key]; ok &&
		cached.token != "" && time.Now().Before(cached.expiresAt) {
		return cached.token, nil
	}

	body, err := json.Marshal(map[string]string{
		"appKey":    strings.TrimSpace(cfg.ClientID),
		"appSecret": strings.TrimSpace(cfg.ClientSecret),
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dingTalkAppTokenEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := newOIDCHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("DingTalk app token request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("DingTalk app token request failed: status=%d", resp.StatusCode)
	}

	var token dingTalkAppTokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&token); err != nil {
		return "", fmt.Errorf("failed to decode DingTalk app token response: %w", err)
	}
	token.AccessToken = strings.TrimSpace(token.AccessToken)
	if token.AccessToken == "" {
		return "", errors.New("DingTalk app token response missing accessToken")
	}

	lifetime := time.Duration(token.ExpireIn) * time.Second
	if lifetime <= 0 {
		lifetime = 2 * time.Hour
	}
	usable := lifetime - dingTalkAppTokenRefreshAdvance
	if usable <= 0 {
		usable = lifetime / 2
	}
	dingTalkAppTokenCache.entries[key] = dingTalkAppTokenCacheEntry{
		token:     token.AccessToken,
		expiresAt: time.Now().Add(usable),
	}
	return token.AccessToken, nil
}

func resolveDingTalkUserIDByUnionID(
	ctx context.Context, appToken string, unionID string,
) (string, error) {
	unionID = strings.TrimSpace(unionID)
	if unionID == "" {
		return "", errors.New("DingTalk unionId is required for department sync")
	}

	endpoint, err := url.Parse(dingTalkUserIDByUnionEndpoint)
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("access_token", appToken)
	endpoint.RawQuery = query.Encode()

	body, err := json.Marshal(map[string]string{"unionid": unionID})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := newOIDCHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("DingTalk unionId lookup failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("DingTalk unionId lookup failed: status=%d", resp.StatusCode)
	}

	var result dingTalkUserIDByUnionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode DingTalk unionId lookup: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("DingTalk unionId lookup failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	result.Result.UserID = strings.TrimSpace(result.Result.UserID)
	if result.Result.UserID == "" {
		return "", errors.New("DingTalk unionId lookup returned empty userid")
	}
	return result.Result.UserID, nil
}

func fetchDingTalkDepartmentIDs(
	ctx context.Context, appToken string, userID string,
) ([]int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("DingTalk userid is required for department sync")
	}

	endpoint, err := url.Parse(dingTalkUserDetailEndpoint)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("access_token", appToken)
	endpoint.RawQuery = query.Encode()

	body, err := json.Marshal(map[string]string{
		"userid":   userID,
		"language": "zh_CN",
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := newOIDCHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("DingTalk user detail request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("DingTalk user detail request failed: status=%d", resp.StatusCode)
	}

	var result dingTalkUserDetailResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode DingTalk user detail: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("DingTalk user detail request failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}

	seen := make(map[int64]struct{}, len(result.Result.DeptIDList))
	departmentIDs := make([]int64, 0, len(result.Result.DeptIDList))
	for _, id := range result.Result.DeptIDList {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		departmentIDs = append(departmentIDs, id)
	}
	sort.Slice(departmentIDs, func(i, j int) bool { return departmentIDs[i] < departmentIDs[j] })
	return departmentIDs, nil
}

func buildDingTalkOrganizationTargets(
	mappings []config.DingTalkDepartmentOrganizationMapping,
	departmentIDs []int64,
) []types.DingTalkOrganizationMembershipTarget {
	departmentSet := make(map[int64]struct{}, len(departmentIDs))
	for _, id := range departmentIDs {
		if id > 0 {
			departmentSet[id] = struct{}{}
		}
	}

	type aggregate struct {
		role        types.OrgMemberRole
		departments map[int64]struct{}
	}
	byOrg := make(map[string]*aggregate)
	for _, mapping := range mappings {
		if _, matched := departmentSet[mapping.DepartmentID]; !matched {
			continue
		}
		orgID := strings.TrimSpace(mapping.OrganizationID)
		if orgID == "" || (mapping.Role != types.OrgRoleViewer && mapping.Role != types.OrgRoleEditor) {
			continue
		}
		item := byOrg[orgID]
		if item == nil {
			item = &aggregate{role: mapping.Role, departments: map[int64]struct{}{}}
			byOrg[orgID] = item
		} else if mapping.Role.HasPermission(item.role) {
			item.role = mapping.Role
		}
		item.departments[mapping.DepartmentID] = struct{}{}
	}

	orgIDs := make([]string, 0, len(byOrg))
	for orgID := range byOrg {
		orgIDs = append(orgIDs, orgID)
	}
	sort.Strings(orgIDs)

	targets := make([]types.DingTalkOrganizationMembershipTarget, 0, len(orgIDs))
	for _, orgID := range orgIDs {
		item := byOrg[orgID]
		deps := make([]int64, 0, len(item.departments))
		for id := range item.departments {
			deps = append(deps, id)
		}
		sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })
		targets = append(targets, types.DingTalkOrganizationMembershipTarget{
			OrganizationID: orgID,
			Role:           item.role,
			DepartmentIDs:  deps,
		})
	}
	return targets
}

func (s *userService) syncDingTalkDepartmentOrganizations(
	ctx context.Context,
	user *types.User,
	tenantID uint64,
	unionID string,
) error {
	cfg := s.config.DingTalkAuth
	if cfg == nil || !cfg.SyncDepartments {
		return nil
	}
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return errors.New("DingTalk department sync requires local user")
	}
	if tenantID == 0 {
		return errors.New("DingTalk department sync requires active tenant")
	}
	syncRepo, ok := s.userRepo.(interfaces.DingTalkOrganizationSyncRepository)
	if !ok {
		return errors.New("DingTalk organization sync repository is unavailable")
	}

	appToken, err := s.fetchDingTalkAppToken(ctx)
	if err != nil {
		return err
	}
	dingUserID, err := resolveDingTalkUserIDByUnionID(ctx, appToken, unionID)
	if err != nil {
		return err
	}
	departmentIDs, err := fetchDingTalkDepartmentIDs(ctx, appToken, dingUserID)
	if err != nil {
		return err
	}
	targets := buildDingTalkOrganizationTargets(cfg.DepartmentOrganizationMappings, departmentIDs)
	return syncRepo.SyncDingTalkOrganizationMemberships(
		ctx,
		user.ID,
		tenantID,
		strings.TrimSpace(cfg.CorpID),
		targets,
	)
}
