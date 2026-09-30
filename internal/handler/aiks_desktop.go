package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const (
	aiksDesktopKBName    = "AIKS Sessions"
	aiksDesktopKeyPrefix = "aiks-desktop:"
	aiksDesktopPairTTL   = 5 * time.Minute
)

type aiksDesktopBootstrapData struct {
	TenantID       uint64 `json:"tenant_id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	APIKey          string `json:"api_key"`
	DisplayName     string `json:"display_name"`
}

type aiksDesktopPairAttempt struct {
	VerifierHash string
	LaunchHash   [32]byte
	ExpiresAt    time.Time
	Approved     *aiksDesktopBootstrapData
}

var aiksDesktopPairs = struct {
	sync.Mutex
	items map[string]*aiksDesktopPairAttempt
}{items: make(map[string]*aiksDesktopPairAttempt)}

type aiksDesktopStartRequest struct {
	VerifierHash string `json:"verifier_hash"`
}

type aiksDesktopApproveRequest struct {
	AttemptID string `json:"attempt_id"`
	Launch    string `json:"launch"`
}

type aiksDesktopExchangeRequest struct {
	AttemptID string `json:"attempt_id"`
	Verifier  string `json:"verifier"`
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func validSHA256Hex(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func expireAIKSDesktopPairsLocked(now time.Time) {
	for id, attempt := range aiksDesktopPairs.items {
		if attempt == nil || !attempt.ExpiresAt.After(now) {
			delete(aiksDesktopPairs.items, id)
		}
	}
}

// AIKSDesktopConnectStart creates a short-lived verifier-bound browser login attempt.
func (h *TenantHandler) AIKSDesktopConnectStart(c *gin.Context) {
	var req aiksDesktopStartRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validSHA256Hex(req.VerifierHash) {
		_ = c.Error(errors.NewValidationError("invalid verifier_hash"))
		return
	}
	attemptID, err := randomHex(24)
	if err != nil {
		_ = c.Error(errors.NewInternalServerError("failed to create desktop login attempt"))
		return
	}
	launch, err := randomHex(32)
	if err != nil {
		_ = c.Error(errors.NewInternalServerError("failed to create desktop login attempt"))
		return
	}
	now := time.Now().UTC()
	attempt := &aiksDesktopPairAttempt{
		VerifierHash: req.VerifierHash,
		LaunchHash:   sha256.Sum256([]byte(launch)),
		ExpiresAt:    now.Add(aiksDesktopPairTTL),
	}
	aiksDesktopPairs.Lock()
	expireAIKSDesktopPairsLocked(now)
	aiksDesktopPairs.items[attemptID] = attempt
	aiksDesktopPairs.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"attempt_id": attemptID,
		"authorize_path": "/api/v1/aiks/desktop/connect/browser?attempt_id=" + attemptID + "&launch=" + launch,
		"expires_at": attempt.ExpiresAt.Unix(),
	})
}

// AIKSDesktopConnectBrowser is a same-origin bridge. It reads the normal web
// login token from localStorage and asks the authenticated approve endpoint to
// provision a scoped Desktop credential. The JWT never leaves the browser.
func (h *TenantHandler) AIKSDesktopConnectBrowser(c *gin.Context) {
	attemptID := c.Query("attempt_id")
	launch := c.Query("launch")
	if !validSHA256Hex(launch) || attemptID == "" {
		c.String(http.StatusBadRequest, "Invalid AIKS Desktop login request")
		return
	}

	launchHash := sha256.Sum256([]byte(launch))
	aiksDesktopPairs.Lock()
	attempt := aiksDesktopPairs.items[attemptID]
	valid := attempt != nil && attempt.ExpiresAt.After(time.Now().UTC()) &&
		subtle.ConstantTimeCompare(attempt.LaunchHash[:], launchHash[:]) == 1
	aiksDesktopPairs.Unlock()
	if !valid {
		c.String(http.StatusGone, "AIKS Desktop login request expired")
		return
	}

	// Values are HTML-escaped even though both are validated opaque hex.
	safeAttempt := html.EscapeString(attemptID)
	safeLaunch := html.EscapeString(launch)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'")
	c.String(http.StatusOK, `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>连接 AIKS Desktop</title>
<style>body{font-family:system-ui,-apple-system,sans-serif;background:#f8fafc;color:#0f172a;display:grid;place-items:center;height:100vh;margin:0}.card{background:#fff;border:1px solid #e2e8f0;border-radius:16px;padding:32px;max-width:520px;box-shadow:0 10px 30px #0f172a12}h1{font-size:22px;margin:0 0 12px}p{line-height:1.7;color:#475569}.ok{color:#047857}.err{color:#b91c1c}</style></head>
<body><div class="card"><h1>连接 AIKS Desktop</h1><p id="status">正在检查 WeKnora 登录状态…</p></div>
<script>
(async()=>{const status=document.getElementById('status');const token=localStorage.getItem('weknora_token');
if(!token){sessionStorage.setItem('aiks_desktop_handoff',location.href);location.replace('/login?desktop=1');return;}
try{const r=await fetch('/api/v1/aiks/desktop/connect/approve',{method:'POST',headers:{'Content-Type':'application/json','Authorization':'Bearer '+token},body:JSON.stringify({attempt_id:'`+safeAttempt+`',launch:'`+safeLaunch+`'})});
if(!r.ok)throw new Error('HTTP '+r.status);status.className='ok';status.textContent='连接成功。AIKS Desktop 已获得当前账号的私有知识库权限，可以关闭此页面。';}
catch(e){status.className='err';status.textContent='连接失败，请回到 AIKS Desktop 重试。';}})();
</script></body></html>`)
}

// AIKSDesktopConnectApprove runs under the normal JWT session.
func (h *TenantHandler) AIKSDesktopConnectApprove(c *gin.Context) {
	var req aiksDesktopApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.AttemptID == "" || !validSHA256Hex(req.Launch) {
		_ = c.Error(errors.NewValidationError("invalid desktop login approval"))
		return
	}

	now := time.Now().UTC()
	launchHash := sha256.Sum256([]byte(req.Launch))
	aiksDesktopPairs.Lock()
	expireAIKSDesktopPairsLocked(now)
	attempt := aiksDesktopPairs.items[req.AttemptID]
	if attempt == nil || subtle.ConstantTimeCompare(attempt.LaunchHash[:], launchHash[:]) != 1 {
		aiksDesktopPairs.Unlock()
		_ = c.Error(errors.NewUnauthorizedError("desktop login attempt expired"))
		return
	}
	aiksDesktopPairs.Unlock()

	user, err := h.userService.GetCurrentUser(c.Request.Context())
	if err != nil || user == nil {
		_ = c.Error(errors.NewUnauthorizedError("login required"))
		return
	}
	data, err := h.ensureAIKSDesktopBootstrap(c.Request.Context(), user)
	if err != nil {
		_ = c.Error(errors.NewInternalServerError("failed to prepare AIKS Desktop").WithDetails(err.Error()))
		return
	}

	aiksDesktopPairs.Lock()
	if current := aiksDesktopPairs.items[req.AttemptID]; current != nil && current.ExpiresAt.After(time.Now().UTC()) {
		current.Approved = data
	}
	aiksDesktopPairs.Unlock()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AIKSDesktopConnectExchange is called only by the native Desktop process.
func (h *TenantHandler) AIKSDesktopConnectExchange(c *gin.Context) {
	var req aiksDesktopExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.AttemptID == "" || req.Verifier == "" || len(req.Verifier) > 256 {
		_ = c.Error(errors.NewValidationError("invalid desktop login exchange"))
		return
	}
	digest := sha256.Sum256([]byte(req.Verifier))
	digestHex := hex.EncodeToString(digest[:])

	aiksDesktopPairs.Lock()
	defer aiksDesktopPairs.Unlock()
	expireAIKSDesktopPairsLocked(time.Now().UTC())
	attempt := aiksDesktopPairs.items[req.AttemptID]
	if attempt == nil || subtle.ConstantTimeCompare([]byte(attempt.VerifierHash), []byte(digestHex)) != 1 {
		_ = c.Error(errors.NewUnauthorizedError("desktop login attempt expired"))
		return
	}
	if attempt.Approved == nil {
		c.JSON(http.StatusAccepted, gin.H{"state": "pending"})
		return
	}
	data := *attempt.Approved
	delete(aiksDesktopPairs.items, req.AttemptID)
	c.JSON(http.StatusOK, gin.H{"state": "connected", "credential": data})
}

// AIKSDesktopBootstrap is useful for authenticated diagnostics and is also the
// single business implementation used by the browser approval flow.
func (h *TenantHandler) AIKSDesktopBootstrap(c *gin.Context) {
	user, err := h.userService.GetCurrentUser(c.Request.Context())
	if err != nil || user == nil {
		_ = c.Error(errors.NewUnauthorizedError("login required"))
		return
	}
	data, err := h.ensureAIKSDesktopBootstrap(c.Request.Context(), user)
	if err != nil {
		_ = c.Error(errors.NewInternalServerError("failed to prepare AIKS Desktop").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *TenantHandler) ensureAIKSDesktopBootstrap(ctx context.Context, user *types.User) (*aiksDesktopBootstrapData, error) {
	if user.TenantID == 0 {
		return nil, fmt.Errorf("personal workspace is required")
	}
	tenant, err := h.service.GetTenantByID(ctx, user.TenantID)
	if err != nil || tenant == nil {
		return nil, fmt.Errorf("home workspace is unavailable")
	}

	homeCtx := context.WithValue(ctx, types.TenantIDContextKey, tenant.ID)
	homeCtx = context.WithValue(homeCtx, types.TenantInfoContextKey, tenant)
	homeCtx = context.WithValue(homeCtx, types.UserIDContextKey, user.ID)

	kbs, err := h.kbService.ListKnowledgeBasesByTenantID(homeCtx, tenant.ID)
	if err != nil {
		return nil, err
	}
	var target *types.KnowledgeBase
	for _, kb := range kbs {
		if kb != nil && kb.Name == aiksDesktopKBName && (kb.CreatorID == "" || kb.CreatorID == user.ID) {
			target = kb
			break
		}
	}
	if target == nil {
		target, err = h.kbService.CreateKnowledgeBase(homeCtx, &types.KnowledgeBase{
			Name:        aiksDesktopKBName,
			Type:        types.KnowledgeBaseTypeDocument,
			Description: "Private AIKS Desktop session archive",
		})
		if err != nil {
			return nil, err
		}
	}

	keyName := aiksDesktopKeyPrefix + user.ID
	keys, err := h.apiKeyService.ListAPIKeys(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}
	var token string
	for _, key := range keys {
		if key == nil || key.Name != keyName || key.IsPlatform() {
			continue
		}
		if key.ExpiresAt != nil && !key.ExpiresAt.After(time.Now().UTC()) {
			continue
		}
		desired := !key.FullAccess &&
			len(key.KnowledgeBaseIDs) == 1 && key.KnowledgeBaseIDs[0] == target.ID &&
			len(key.Capabilities) == 1 && key.Capabilities[0] == string(types.APIKeyCapabilityRetrieve)
		if !desired {
			// Preserve the decrypted token returned by ListAPIKeys. The update
			// operation changes only scope metadata and may return a projection
			// whose secret field is not useful to callers.
			existingToken := key.APIKey
			if _, updateErr := h.apiKeyService.UpdateAPIKey(ctx, interfaces.TenantAPIKeyUpdateRequest{
				TenantID: tenant.ID, APIKeyID: key.ID, Name: keyName,
				FullAccess: false, KnowledgeBaseIDs: []string{target.ID},
				Capabilities: []string{string(types.APIKeyCapabilityRetrieve)},
			}); updateErr != nil {
				return nil, updateErr
			}
			token = existingToken
		} else {
			token = key.APIKey
		}
		if token != "" {
			break
		}
	}
	if token == "" {
		created, createErr := h.apiKeyService.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
			TenantID: tenant.ID, ScopeType: types.APIKeyScopeTenant, Name: keyName,
			FullAccess: false, KnowledgeBaseIDs: []string{target.ID},
			Capabilities: []string{string(types.APIKeyCapabilityRetrieve)},
		})
		if createErr != nil {
			return nil, createErr
		}
		token = created.Token
	}
	if token == "" {
		return nil, fmt.Errorf("desktop credential is unavailable")
	}
	display := strings.TrimSpace(user.Username)
	if display == "" {
		display = strings.TrimSpace(user.Email)
	}
	return &aiksDesktopBootstrapData{
		TenantID: tenant.ID, KnowledgeBaseID: target.ID, APIKey: token, DisplayName: display,
	}, nil
}
