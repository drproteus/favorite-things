# Favorite Things

A personal inventory and collection app. Catalog anything you collect (consoles, albums, t-shirts, ...), attach photos, tag and rate items, and choose whether each one is private or public.

- **Backend:** Go (`net/http`), PostgreSQL (`pgx`), S3 or MinIO for images, Google and GitHub OAuth
- **Frontend:** React 19, TypeScript, Vite, Tailwind CSS, TanStack Query

## Features

- Sign in with Google or GitHub (no passwords)
- Create, edit and delete items with a title, description, category, tags and photos (0 to 20 per item)
- Public or private visibility, toggled from the item page or the edit form
- Explore page for public items, with search and category filters
- Share public items with a copy-link button
- 1 to 5 star ratings: each signed-in user can rate any item they can see, and the average is shown
- Responsive layout with light and dark mode

## Project layout

```
backend/            Go API
  cmd/server/       entrypoint
  internal/
    auth/           OAuth login, server-side sessions, middleware
    config/         environment configuration
    database/       pgx pool and embedded SQL migrations
    items/          items, tags, images, ratings: store and HTTP handlers
    server/         router and SPA fallback
    storage/        S3/MinIO client and presigned URLs
    httpx/          JSON helpers
frontend/           React app (Vite)
docker-compose.yml  Postgres + MinIO (+ optional full app)
Dockerfile          builds the web app and API into one image
```

## Quick start (development)

Prerequisites: Go 1.26+, Node 24+, Docker.

1. **Start Postgres and MinIO**

   ```sh
   docker compose up -d postgres minio
   ```

   Postgres is exposed on host port **5433** (5432 is commonly taken by a local install). MinIO's console is at http://localhost:9001 (login `minioadmin` / `minioadmin`). The app creates the S3 bucket on startup.

2. **Configure OAuth** (see below), then copy the env template:

   ```sh
   cp .env.example backend/.env   # or .env in the repo root; see note below
   ```

   The backend loads `.env` from its working directory. Run it from `backend/`, or export the variables yourself.

3. **Run the API** (port 8080):

   ```sh
   cd backend
   go run ./cmd/server
   ```

4. **Run the web app** (port 5173, proxies `/api` to the API):

   ```sh
   cd frontend
   npm install
   npm run dev
   ```

   Open http://localhost:5173.

## OAuth setup

Redirect URIs use `PUBLIC_URL`, which defaults to `http://localhost:8080`.

- **Google:** Google Cloud Console → APIs & Services → Credentials → Create OAuth client (Web application). Add the authorized redirect URI `http://localhost:8080/api/auth/google/callback`. Set `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET`.
- **GitHub:** Settings → Developer settings → OAuth Apps → New OAuth App. Set the authorization callback URL to `http://localhost:8080/api/auth/github/callback`. Set `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET`.

A provider button only appears on the login page when its client ID is set.

Accounts are keyed by provider and provider user ID. If a provider reports a verified email that already belongs to another account, the two are linked. Unverified emails are never used for linking.

## Running everything in Docker

```sh
docker compose --profile full up --build
```

This starts Postgres, MinIO and the app at http://localhost:8080. Put your OAuth credentials in a `.env` file in the repo root, or set them in your shell. For the OAuth redirect URIs, use `http://localhost:8080/api/auth/<provider>/callback`.

## Production notes

- Serve over HTTPS and set `COOKIE_SECURE=true`. Set `PUBLIC_URL` and `FRONTEND_URL` to your public origins. When the app is served by the Go binary, `FRONTEND_URL` is the same as `PUBLIC_URL`.
- Images are served through presigned URLs that expire after one hour, so the bucket can stay private. Set `S3_PUBLIC_ENDPOINT` if the browser needs a different host from the one the API uses to reach storage.
- For AWS S3, set `S3_ENDPOINT` to empty (or your region's endpoint), `S3_PATH_STYLE=false`, and create the bucket yourself. `EnsureBucket` only creates buckets in `us-east-1`-style setups.
- Migrations run automatically at startup and are tracked in `schema_migrations`.

## API overview

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| GET | `/api/auth/providers` | | Enabled sign-in providers |
| GET | `/api/auth/{google\|github}/login` | | Start OAuth |
| GET | `/api/auth/{provider}/callback` | | OAuth callback (sets session cookie) |
| POST | `/api/auth/logout` | | End session |
| GET | `/api/me` | yes | Current user |
| GET | `/api/items?scope=mine\|public&q&tag&category&visibility&limit&offset` | scope=mine: yes | List items |
| POST | `/api/items` | yes | Create item |
| GET | `/api/items/{id}` | public or owner | Get item |
| PUT | `/api/items/{id}` | owner | Update item (including visibility) |
| DELETE | `/api/items/{id}` | owner | Delete item and its photos |
| POST | `/api/items/{id}/images` | owner | Upload photo (multipart field `file`) |
| DELETE | `/api/items/{id}/images/{imageId}` | owner | Remove photo |
| PUT | `/api/items/{id}/rating` | viewer of item | Set your rating `{ "rating": 1-5 }` |
| DELETE | `/api/items/{id}/rating` | yes | Clear your rating |
| GET | `/api/tags` | yes | Your tags (for autocomplete) |

## Tests and checks

```sh
cd backend && go vet ./... && go build ./...
cd frontend && npm run build
```
