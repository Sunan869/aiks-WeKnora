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
  "channel": "aiks"
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
