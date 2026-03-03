# CodeLens

CodeLens now uses a single authentication and inference path:

- Provider: `Codex` only
- Auth: `OAuth Authorization Code + PKCE`
- Global config file: `~/.codelens/config.json`

## Quickstart (3 steps)

1. Build:

```bash
go build -o codelens ./cmd/codelens
```

2. Authenticate:

```bash
./codelens configure
```

Alternative (no browser redirect, direct config write/update):

```bash
./codelens configure --oauth-token <token> --client-id app_EMoamEEZ73f0CkXaXp7hrann --model codex --concurrency 5
```

3. Run:

```bash
./codelens
```

Debug request/response payloads (without tokens):

```bash
./codelens --debug
```

If config is missing, commands that require model access stop with:

```text
Configuración no encontrada en ~/.codelens/config.json. Ejecutá: codelens configure
```

## OAuth env overrides (optional)

- `CODEX_OAUTH_CLIENT_ID`
- `CODEX_OAUTH_AUTHORIZE_URL`
- `CODEX_OAUTH_TOKEN_URL`
- `CODEX_OAUTH_SCOPES`
- `CODEX_OAUTH_CALLBACK_ADDR`
- `CODEX_OAUTH_REDIRECT_URL`
- `CODEX_OAUTH_ORIGINATOR`
