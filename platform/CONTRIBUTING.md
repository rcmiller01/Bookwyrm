# Contributing

## Tooling

- Go 1.23 required.
- Node 20 required for `app/backend/web`.

## Local Validation

Run before opening a PR:

```bash
# Go tests
cd metadata-service && go test ./... -count=1
cd ../indexer-service && go test ./... -count=1
cd ../app/backend && go test ./... -count=1

# Frontend lint/tests/build
cd web
npm ci
npm run lint
npm test
npm run build
```

## Pull Request Expectations

- Keep changes scoped and additive.
- Avoid introducing parallel config models where one already exists.
- Include migration notes when schema changes are introduced.
- For operational changes, include docs updates in the same PR.
