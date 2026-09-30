package router

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const (
	aiksDesktopKnowledgeBaseName = "AIKS Sessions"
	aiksDesktopKeyPrefix         = "AIKS Desktop - "
	aiksDesktopPairTTL           = 5 * time.Minute
)

type aiksDesktopData struct {
	TenantID        uint64   `json:"tenant_id"`
	KnowledgeBaseID string   `json:"knowledge_base_id"`
	KnowledgeBase   string   `json:"knowledge_base"`
	APIKey           string   `json:"api_key"`
	DisplayName      string   `json:"display_name,omitempty"`
	Capabilities     []string `json:"capabilities,omitempty"`
}

type aiksDesktopPair struct {
	VerifierHash string
	LaunchHash   [32]byte
	ExpiresAt    time.Time
	Approved     *aiksDesktopData
}

var aiksDesktopPairs = struct {
	sync.Mutex
	items map[string]*aiksDesktopPair
}{items: make(map[string]*aiksDesktopPair)}

func aiksRandomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func aiksValidSHA256Hex(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func expireAIKSPairsLocked(now time.Time) {
	for id, attempt := range aiksDesktopPairs.items {
		if attempt == nil || !attempt.ExpiresAt.After(now) {
			delete(aiksDesktopPairs.items, id)
		}
	}
}

func ensureAIKSDesktop(
	c *gin.Context,
	kbService interfaces.KnowledgeBaseService,
	modelService interfaces.ModelService,
	apiKeyService interfaces.TenantAPIKeyService,
	userService interfaces.UserService,
) (*aiksDesktopData, int, string) {
	ctx := c.Request.Context()
	tenantID := types.MustTenantIDFromContext(ctx)
	userID, ok := types.UserIDFromContext(ctx)
	if !ok || strings.TrimSpace(userID) == "" || types.IsSyntheticUserID(userID) {
		return nil, http.StatusUnauthorized, "authenticated user required"
	}

	kbs, err := kbService.ListKnowledgeBases(ctx)
	if err != nil {
		return nil, http.StatusInternalServerError, "failed to list knowledge bases"
	}

	var kb *types.KnowledgeBase
	for _, candidate := range kbs {
		if candidate == nil || candidate.Name != aiksDesktopKnowledgeBaseName {
			continue
		}
		if candidate.CreatorID == userID {
			kb = candidate
			break
		}
		if kb == nil && candidate.CreatorID == "" {
			kb = candidate
		}
	}

	if kb == nil {
		models, modelErr := modelService.ListModels(ctx)
		if modelErr != nil {
			return nil, http.StatusInternalServerError, "failed to list models"
		}
		var embeddingID, summaryID string
		for _, model := range models {
			if model == nil || model.Status != types.ModelStatusActive {
				continue
			}
			switch model.Type {
			case types.ModelTypeEmbedding:
				if embeddingID == "" || model.IsDefault {
					embeddingID = model.ID
				}
			case types.ModelTypeKnowledgeQA:
				if summaryID == "" || model.IsDefault {
					summaryID = model.ID
				}
			}
		}
		if embeddingID == "" {
			return nil, http.StatusConflict, "embedding model is not configured"
		}
		kb, err = kbService.CreateKnowledgeBase(ctx, &types.KnowledgeBase{
			Name:             aiksDesktopKnowledgeBaseName,
			Type:             types.KnowledgeBaseTypeDocument,
			Description:      "Private AIKS desktop session archive",
			EmbeddingModelID: embeddingID,
			SummaryModelID:   summaryID,
		})
		if err != nil {
			return nil, http.StatusInternalServerError, "failed to create AIKS Sessions knowledge base"
		}
	}

	keyName := aiksDesktopKeyPrefix + userID
	keys, err := apiKeyService.ListAPIKeys(ctx, tenantID)
	if err != nil {
		return nil, http.StatusInternalServerError, "failed to list desktop credentials"
	}

	var desktopKey *types.TenantAPIKey
	for _, candidate := range keys {
		if candidate != nil && candidate.Name == keyName && candidate.RevokedAt == nil {
			desktopKey = candidate
			break
		}
	}

	var token string
	if desktopKey == nil {
		created, createErr := apiKeyService.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
			TenantID:         tenantID,
			ScopeType:        types.APIKeyScopeTenant,
			Name:             keyName,
			FullAccess:       false,
			KnowledgeBaseIDs: []string{kb.ID},
			Capabilities:     []string{string(types.APIKeyCapabilityRetrieve)},
		})
		if createErr != nil {
			return nil, http.StatusInternalServerError, "failed to create desktop credential"
		}
		token = created.Token
	} else {
		token = desktopKey.APIKey
		needsUpdate := desktopKey.FullAccess ||
			len(desktopKey.KnowledgeBaseIDs) != 1 ||
			desktopKey.KnowledgeBaseIDs[0] != kb.ID ||
			len(desktopKey.Capabilities) != 1 ||
			desktopKey.Capabilities[0] != string(types.APIKeyCapabilityRetrieve)
		if needsUpdate {
			if _, updateErr := apiKeyService.UpdateAPIKey(ctx, interfaces.TenantAPIKeyUpdateRequest{
				TenantID:         tenantID,
				APIKeyID:         desktopKey.ID,
				Name:             keyName,
				FullAccess:       false,
				KnowledgeBaseIDs: []string{kb.ID},
				Capabilities:     []string{string(types.APIKeyCapabilityRetrieve)},
				ExpiresAt:        desktopKey.ExpiresAt,
			}); updateErr != nil {
				return nil, http.StatusInternalServerError, "failed to update desktop credential"
			}
		}
	}

	if strings.TrimSpace(token) == "" {
		return nil, http.StatusConflict, "desktop credential cannot be recovered; revoke it and retry bootstrap"
	}

	displayName := userID
	if userService != nil {
		if user, userErr := userService.GetCurrentUser(ctx); userErr == nil && user != nil {
			if value := strings.TrimSpace(user.Username); value != "" {
				displayName = value
			} else if value := strings.TrimSpace(user.Email); value != "" {
				displayName = value
			}
		}
	}

	return &aiksDesktopData{
		TenantID:        tenantID,
		KnowledgeBaseID: kb.ID,
		KnowledgeBase:   kb.Name,
		APIKey:           token,
		DisplayName:      displayName,
		Capabilities:     []string{string(types.APIKeyCapabilityRetrieve)},
	}, http.StatusOK, ""
}

// RegisterAIKSDesktopRoutes owns both the browser pairing handshake and the
// authenticated idempotent bootstrap. Public handshake endpoints are still
// verifier-bound; approve/bootstrap require the normal JWT session.
func RegisterAIKSDesktopRoutes(
	r *gin.RouterGroup,
	kbService interfaces.KnowledgeBaseService,
	modelService interfaces.ModelService,
	apiKeyService interfaces.TenantAPIKeyService,
	userService interfaces.UserService,
	g *rbacGuards,
) {
	if kbService == nil || modelService == nil || apiKeyService == nil {
		return
	}

	r.POST("/aiks/desktop/connect/start", func(c *gin.Context) {
		var req struct {
			VerifierHash string `json:"verifier_hash"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || !aiksValidSHA256Hex(req.VerifierHash) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid verifier_hash"})
			return
		}
		attemptID, err := aiksRandomHex(24)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to create desktop login"})
			return
		}
		launch, err := aiksRandomHex(32)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to create desktop login"})
			return
		}
		now := time.Now().UTC()
		attempt := &aiksDesktopPair{
			VerifierHash: req.VerifierHash,
			LaunchHash:   sha256.Sum256([]byte(launch)),
			ExpiresAt:    now.Add(aiksDesktopPairTTL),
		}
		aiksDesktopPairs.Lock()
		expireAIKSPairsLocked(now)
		aiksDesktopPairs.items[attemptID] = attempt
		aiksDesktopPairs.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"attempt_id": attemptID,
			"authorize_path": "/api/v1/aiks/desktop/connect/browser?attempt_id=" + attemptID + "&launch=" + launch,
			"expires_at": attempt.ExpiresAt.Unix(),
		})
	})

	r.GET("/aiks/desktop/connect/browser", func(c *gin.Context) {
		attemptID := c.Query("attempt_id")
		launch := c.Query("launch")
		if attemptID == "" || !aiksValidSHA256Hex(launch) {
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

		safeAttempt := html.EscapeString(attemptID)
		safeLaunch := html.EscapeString(launch)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'")
		c.String(http.StatusOK, `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>连接 AIKS Desktop</title><style>body{font-family:system-ui,-apple-system,sans-serif;background:#f8fafc;color:#0f172a;display:grid;place-items:center;height:100vh;margin:0}.card{background:#fff;border:1px solid #e2e8f0;border-radius:16px;padding:32px;max-width:520px;box-shadow:0 10px 30px #0f172a12}h1{font-size:22px;margin:0 0 12px}p{line-height:1.7;color:#475569}.ok{color:#047857}.err{color:#b91c1c}</style></head><body><div class="card"><h1>连接 AIKS Desktop</h1><p id="status">正在检查 WeKnora 登录状态…</p></div><script>(async()=>{const s=document.getElementById('status');const t=localStorage.getItem('weknora_token');if(!t){sessionStorage.setItem('aiks_desktop_handoff',location.href);location.replace('/login?desktop=1');return;}try{const r=await fetch('/api/v1/aiks/desktop/connect/approve',{method:'POST',headers:{'Content-Type':'application/json','Authorization':'Bearer '+t},body:JSON.stringify({attempt_id:'`+safeAttempt+`',launch:'`+safeLaunch+`'})});if(!r.ok)throw new Error();s.className='ok';s.textContent='连接成功，可以关闭此页面并返回 AIKS Desktop。';}catch(e){s.className='err';s.textContent='连接失败，请回到 AIKS Desktop 重试。';}})();</script></body></html>`)
	})

	r.POST("/aiks/desktop/connect/approve", g.Viewer(), func(c *gin.Context) {
		var req struct {
			AttemptID string `json:"attempt_id"`
			Launch    string `json:"launch"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.AttemptID == "" || !aiksValidSHA256Hex(req.Launch) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid desktop login approval"})
			return
		}
		launchHash := sha256.Sum256([]byte(req.Launch))
		aiksDesktopPairs.Lock()
		expireAIKSPairsLocked(time.Now().UTC())
		attempt := aiksDesktopPairs.items[req.AttemptID]
		valid := attempt != nil && subtle.ConstantTimeCompare(attempt.LaunchHash[:], launchHash[:]) == 1
		aiksDesktopPairs.Unlock()
		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "desktop login attempt expired"})
			return
		}
		data, code, message := ensureAIKSDesktop(c, kbService, modelService, apiKeyService, userService)
		if data == nil {
			c.JSON(code, gin.H{"success": false, "message": message})
			return
		}
		aiksDesktopPairs.Lock()
		if current := aiksDesktopPairs.items[req.AttemptID]; current != nil && current.ExpiresAt.After(time.Now().UTC()) {
			current.Approved = data
		}
		aiksDesktopPairs.Unlock()
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.POST("/aiks/desktop/connect/exchange", func(c *gin.Context) {
		var req struct {
			AttemptID string `json:"attempt_id"`
			Verifier  string `json:"verifier"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.AttemptID == "" || req.Verifier == "" || len(req.Verifier) > 256 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid desktop login exchange"})
			return
		}
		digest := sha256.Sum256([]byte(req.Verifier))
		digestHex := hex.EncodeToString(digest[:])
		aiksDesktopPairs.Lock()
		defer aiksDesktopPairs.Unlock()
		expireAIKSPairsLocked(time.Now().UTC())
		attempt := aiksDesktopPairs.items[req.AttemptID]
		if attempt == nil || subtle.ConstantTimeCompare([]byte(attempt.VerifierHash), []byte(digestHex)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "desktop login attempt expired"})
			return
		}
		if attempt.Approved == nil {
			c.JSON(http.StatusAccepted, gin.H{"state": "pending"})
			return
		}
		data := *attempt.Approved
		delete(aiksDesktopPairs.items, req.AttemptID)
		c.JSON(http.StatusOK, gin.H{"state": "connected", "credential": data})
	})

	r.POST("/aiks/desktop/bootstrap", g.Viewer(), func(c *gin.Context) {
		data, code, message := ensureAIKSDesktop(c, kbService, modelService, apiKeyService, userService)
		if data == nil {
			extra := gin.H{"success": false, "message": message}
			if code == http.StatusConflict && message == "embedding model is not configured" {
				extra["code"] = "AIKS_EMBEDDING_MODEL_REQUIRED"
			}
			c.JSON(code, extra)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
	})
}
