# AIKS DingTalk login

The team deployment authenticates users in WeKnora, not in `aiks-service`.

## Environment

```bash
DINGTALK_AUTH_ENABLE=true
DINGTALK_AUTH_CLIENT_ID=dingxxxxxxxx
DINGTALK_AUTH_CLIENT_SECRET=...
DINGTALK_AUTH_CORP_ID=dingxxxxxxxx
DINGTALK_AUTH_PROVIDER_DISPLAY_NAME=钉钉
```

Register this callback in DingTalk's “钉钉登录与分享” configuration:

```text
https://<weknora-host>/api/v1/auth/dingtalk/callback
```

Start login at:

```text
GET /api/v1/auth/dingtalk/start
```

The backend exchanges DingTalk `authCode` for a user access token, requests
`/v1.0/contact/users/me`, then uses the existing WeKnora user/workspace/token
pipeline. A signed state plus HttpOnly nonce cookie binds the callback to the
initiating browser.

The stable local identity is `(provider=dingtalk, subject)`, using DingTalk
`unionId` first and `openId` only as fallback. Email is only a first-login
account-linking hint. If the API does not expose email, a reserved
`@external.invalid` address derived from the stable subject satisfies the
existing non-null user schema without pretending it is a real mailbox.
