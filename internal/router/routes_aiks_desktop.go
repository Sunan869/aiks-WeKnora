package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const (
	aiksDesktopKnowledgeBaseName = "AIKS Sessions"
	aiksDesktopKeyPrefix         = "AIKS Desktop - "
)

// RegisterAIKSDesktopRoutes exposes a JWT-only bootstrap surface for AIKS Desktop.
// It is intentionally not registered with apiKeyRoute, so tenant/platform API keys
// remain default-denied by the global API-key authorizer.
func RegisterAIKSDesktopRoutes(
	r *gin.RouterGroup,
	kbService interfaces.KnowledgeBaseService,
	modelService interfaces.ModelService,
	apiKeyService interfaces.TenantAPIKeyService,
	g *rbacGuards,
) {
	if kbService == nil || modelService == nil || apiKeyService == nil {
		return
	}
	r.POST("/aiks/desktop/bootstrap", g.Viewer(), func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := types.MustTenantIDFromContext(ctx)
		userID, ok := types.UserIDFromContext(ctx)
		if !ok || strings.TrimSpace(userID) == "" || types.IsSyntheticUserID(userID) {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "authenticated user required"})
			return
		}

		kbs, err := kbService.ListKnowledgeBases(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to list knowledge bases"})
			return
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
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to list models"})
				return
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
				c.JSON(http.StatusConflict, gin.H{
					"success": false,
					"message": "embedding model is not configured",
					"code": "AIKS_EMBEDDING_MODEL_REQUIRED",
				})
				return
			}
			kb, err = kbService.CreateKnowledgeBase(ctx, &types.KnowledgeBase{
				Name:             aiksDesktopKnowledgeBaseName,
				Type:             types.KnowledgeBaseTypeDocument,
				Description:      "Private AIKS desktop session archive",
				EmbeddingModelID: embeddingID,
				SummaryModelID:   summaryID,
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to create AIKS Sessions knowledge base"})
				return
			}
		}

		keyName := aiksDesktopKeyPrefix + userID
		keys, err := apiKeyService.ListAPIKeys(ctx, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to list desktop credentials"})
			return
		}

		var desktopKey *types.TenantAPIKey
		for _, candidate := range keys {
			if candidate != nil && candidate.Name == keyName && candidate.RevokedAt == nil {
				desktopKey = candidate
				break
			}
		}

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
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to create desktop credential"})
				return
			}
			desktopKey = created.APIKey
			desktopKey.APIKey = created.Token
		} else {
			needsUpdate := desktopKey.FullAccess || len(desktopKey.KnowledgeBaseIDs) != 1 ||
				desktopKey.KnowledgeBaseIDs[0] != kb.ID ||
				len(desktopKey.Capabilities) != 1 ||
				desktopKey.Capabilities[0] != string(types.APIKeyCapabilityRetrieve)
			if needsUpdate {
				desktopKey, err = apiKeyService.UpdateAPIKey(ctx, interfaces.TenantAPIKeyUpdateRequest{
					TenantID:         tenantID,
					APIKeyID:         desktopKey.ID,
					Name:             keyName,
					FullAccess:       false,
					KnowledgeBaseIDs: []string{kb.ID},
					Capabilities:     []string{string(types.APIKeyCapabilityRetrieve)},
					ExpiresAt:        desktopKey.ExpiresAt,
				})
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to update desktop credential"})
					return
				}
			}
		}

		if strings.TrimSpace(desktopKey.APIKey) == "" {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "desktop credential cannot be recovered; revoke it and retry bootstrap",
				"code": "AIKS_DESKTOP_CREDENTIAL_UNAVAILABLE",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"tenant_id":         tenantID,
				"knowledge_base_id": kb.ID,
				"knowledge_base":    kb.Name,
				"api_key":           desktopKey.APIKey,
				"capabilities":      []string{string(types.APIKeyCapabilityRetrieve)},
			},
		})
	})
}
