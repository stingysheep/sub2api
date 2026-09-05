# Local Demo

This directory is a local-only mock API for manually checking the administrator groups page.

It uses synthetic data with the same response shapes as the frontend API. One OpenAI account belongs to two groups so the multi-group behavior can be tested. Changes are held in the mock process memory and are lost when the process stops.

Run the mock API from the repository root:

```powershell
node tools/local-demo/mock-server.mjs
```

In another terminal, run the frontend with the existing Vite config pointed at the mock API:

```powershell
$env:VITE_DEV_PROXY_TARGET = 'http://127.0.0.1:4174'
$env:VITE_DEV_PORT = '4173'
node frontend/node_modules/vite/bin/vite.js --config frontend/vite.config.ts --host 127.0.0.1
```

Open `http://127.0.0.1:4173/login`, then use any email and password. The mock login accepts them and creates a local administrator session. After login, open `/admin/groups`.

This setup does not read, write, build, or send requests to the production server.
