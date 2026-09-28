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
