# AIKS integration

This fork keeps WeKnora as the team knowledge, authorization and RAG system.
AIKS remains responsible for collecting local AI sessions and converts an accepted
session to sanitized Markdown before calling WeKnora's existing manual-knowledge API.

## Initial contract

AIKS creates knowledge through the existing endpoint:

```http
POST /api/v1/knowledge-bases/{knowledge_base_id}/knowledge/manual
X-API-Key: <scoped key>
Content-Type: application/json
```

```json
{
  "title": "Session title",
  "content": "# Session title\n...",
  "status": "publish",
  "channel": "aiks",
  "external_id": "aiks-<sha256(source + session-id)>"
}
```

Subsequent revisions use the returned knowledge ID:

```http
PUT /api/v1/knowledge/manual/{knowledge_id}
```

The first integration deliberately reuses upstream APIs instead of adding an
AIKS-specific ingestion endpoint. The AIKS service owns the
`(source, external_session_id) -> knowledge_id` mapping.

## Desktop automatic bootstrap

The normal AIKS Desktop team flow does not require an operator to pre-create a
knowledge base or paste a per-user API key.

1. Desktop calls `POST /api/v1/aiks/desktop/connect/start` with a SHA-256
   verifier hash and opens the returned same-origin `authorize_path`.
2. The browser uses the normal WeKnora login session (including DingTalk) to
   approve the pairing. The user's JWT remains in the browser.
3. Approval creates or reuses the caller's private `AIKS Sessions` knowledge
   base. An active embedding model is required when the KB must be created.
4. WeKnora creates or repairs an `AIKS Desktop - <user>` tenant API key scoped
   to that KB with the `retrieve` capability.
5. Native Desktop exchanges the one-time verifier for
   `tenant_id`, `knowledge_base_id`, `knowledge_base`, `api_key`,
   `display_name`, and `capabilities`, then uses those values to bootstrap
   the AIKS collector. The WebView never receives the API key.

`POST /api/v1/aiks/desktop/bootstrap` exposes the same idempotent provisioning
for an already authenticated browser session. Responses that can contain the
Desktop API key are marked `Cache-Control: no-store`.

## Ownership boundary

WeKnora owns user/workspace authentication, RBAC, sharing, audit, document
management, RAG, vector retrieval and Wiki behavior.

AIKS owns provider discovery, session normalization, secret redaction,
Session-to-Markdown rendering and delivery/retry state.

## Planned fork-only changes

1. DingTalk enterprise login adapter or an OIDC bridge deployment profile.
2. Finer sharing only if the existing workspace / knowledge-base sharing model
   is insufficient for AIKS product requirements.
3. AIKS branding/navigation after the ingestion proof-of-concept is green.

Do not duplicate WeKnora RBAC, audit, RAG, vector-search or Wiki logic in
`aiks-service`.


## Stable source identity

The fork extends manual-knowledge metadata with an optional `external_id`.
It is accepted only for the `aiks` channel and must use the fixed
`aiks-` + 64 lowercase hex format. Updates preserve the identifier and reject
attempts to change it. This keeps AIKS source identity server-side without
putting raw local Session IDs into WeKnora metadata.


## AIKS manual payload budget

Interactive/manual knowledge keeps the upstream 200,000-character limit.
A request using `channel: "aiks"` must include a valid stable
`external_id` and may carry up to 2,000,000 characters. This larger budget
is isolated to the AIKS ingestion contract so normal editor/API behavior is
unchanged.


## Idempotent create / replay

For AIKS manual ingestion, `external_id` is the source identity inside the
destination knowledge base. A repeated POST with the same identifier:

- returns the existing knowledge immediately when title/content/status are unchanged and the row is healthy;
- routes changed content to `UpdateManualKnowledge`;
- routes a previously failed row through update/reprocessing instead of falsely acknowledging it as complete.

This closes the normal sequential retry window where WeKnora committed a POST
but the caller lost the HTTP response before persisting the returned knowledge ID.


## Team identity and sharing extensions

The AIKS fork keeps upstream workspace/organization sharing and adds two
enterprise-oriented extensions:

- DingTalk login: `/api/v1/auth/dingtalk/start` uses DingTalk OAuth, binds a
  stable external identity (unionId preferred, openId fallback), and then issues
  normal WeKnora access/refresh tokens.
- Direct user KB shares: owners can grant an existing active account
  `viewer` or `editor` permission without creating a one-user organization.

Direct user sharing endpoints:

```text
POST   /api/v1/knowledge-bases/{kb_id}/user-shares
GET    /api/v1/knowledge-bases/{kb_id}/user-shares
GET    /api/v1/knowledge-bases/{kb_id}/user-shares/candidates?q=...
PUT    /api/v1/knowledge-bases/{kb_id}/user-shares/{share_id}
DELETE /api/v1/knowledge-bases/{kb_id}/user-shares/{share_id}
```

A direct grant follows the authenticated user identity across that user's active
workspaces. The effective permission is still capped by the caller's current
tenant role, so a tenant Viewer remains read-only even when the direct grant is
`editor`. Organization shares continue to use the existing three-dimensional
cap and are not changed by this extension.


### Privacy boundary for collected sessions

Direct user grants do not make a shared WeKnora workspace private internally:
WeKnora still treats the workspace/tenant as its primary resource boundary.
Therefore AIKS team ingestion must route each employee's sessions into that
employee's own WeKnora workspace (or another workspace dedicated exclusively to
that employee). A single company-wide ingestion KB is only suitable for a
single-user proof of concept or intentionally shared data.

The collector identity-routing layer is responsible for binding an authenticated
AIKS uploader to the correct WeKnora workspace and KB. Sharing is applied only
after this private placement.
