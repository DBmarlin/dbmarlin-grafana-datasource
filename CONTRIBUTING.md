# Contributing

## Requirements

- Node.js 22 or later (see `.nvmrc`) and npm
- Go (the version in `go.mod`) and [Mage](https://magefile.org)
- Docker, to run a local Grafana
- A DBmarlin 6.12 or later server to test against

## Backend

```bash
go test ./pkg/...
mage -v            # build the backend binaries for Linux, Windows and macOS
mage -l            # list the other targets
```

## Frontend

```bash
npm install
npm run dev        # build in development mode and watch
npm run build      # production build
npm run typecheck
npm run lint       # or lint:fix
npm run test:ci    # Jest
```

## Run it in Grafana

```bash
npm run server     # Grafana in Docker with the plugin loaded (unsigned plugins allowed)
```

Set `DBMARLIN_URL`, `DBMARLIN_USER` and `DBMARLIN_PASSWORD` to point the provisioned data source at your DBmarlin server. Set `GRAFANA_VERSION` to test another Grafana version.

## End-to-end tests

```bash
npm run server     # in one terminal
npm run e2e        # in another (Playwright)
```

## Releasing

The plugin has its own version, independent of DBmarlin. It is set in `package.json` and follows semantic versioning. Update `CHANGELOG.md` for each release and keep the minimum DBmarlin and Grafana versions in `src/README.md` and `src/plugin.json` accurate.

Signing is done only for a reviewed release:

```bash
export GRAFANA_ACCESS_POLICY_TOKEN=<token with the plugins:write scope>
npm run sign
```
