# Favorite Things

A personal inventory and collection app. Catalog anything you collect (consoles, albums, t-shirts, ...), attach photos, tag and rate items, and choose whether each one is private or public.

- **Backend:** Go (`net/http`), PostgreSQL (`pgx`), S3 or MinIO for images, Google and GitHub OAuth
- **Frontend:** React 19, TypeScript, Vite, Tailwind CSS, TanStack Query

## Features

- Sign in with Google or GitHub (no passwords)
- Create, edit and delete items with a title, description, #hashtag tags and photos (0 to 20 per item)
- Folders: every user has a root folder, and folders nest under it to any depth. Each item sits in one folder. Folders can be moved, renamed and set public or private
- A private folder hides everything inside it, even items marked public
- Deleting a folder asks whether to delete its contents or move them up one level. Deleting everything is the default
- Public or private visibility, toggled from the item page or the edit form
- Explore page for public items, with search and tag filters
- Drag-and-drop or browse to add photos
- Tags are #hashtags in the description, Twitter-style, so they need no separate management
- Share public items with a copy-link button
- 1 to 5 star rating on each item, set by its owner and shown to anyone who can see the item
- Responsive layout with light and dark mode

## Project layout

```
backend/            Go API
  cmd/server/       entrypoint
  internal/
    auth/           OAuth login, server-side sessions, middleware
    config/         environment configuration
    database/       pgx pool and embedded SQL migrations
    collections/    folder tree: store and HTTP handlers
    items/          items, hashtags, images, ratings: store and HTTP handlers
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

2. **Configure OAuth** (see below), then copy the env template to the repo root:

   ```sh
   cp .env.example .env
   ```

   The API reads `.env` from the repo root whether you run it from the root or from `backend/`. Variables set in your shell take precedence over the file.

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

> **Schema note:** all migrations are consolidated into `backend/internal/database/migrations/001_init.sql`. A database created before this change has the old migration records and must be reset: `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`. Then the API applies the new schema on start.

## Admin access

Admin access is granted from the command line, not the web UI. The person must sign in at least once first, so their account exists.

Run these from the `backend/` directory. They read the same `.env` as the API and run migrations automatically.

```sh
go run ./cmd/admin list                       # list users, IDs and admin status
go run ./cmd/admin grant  you@example.com     # grant admin by email
go run ./cmd/admin revoke you@example.com     # revoke admin
go run ./cmd/admin grant  <user-id>           # or by user ID (use this if the account has no email)
```

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
| GET | `/api/items?scope=mine\|public&q&tag&collection&visibility&limit&offset` | scope=mine: yes | List items (`collection` limits mine to one folder) |
| POST | `/api/items` | yes | Create item |
| GET | `/api/items/{id}` | public or owner | Get item |
| PUT | `/api/items/{id}` | owner | Update item (including visibility) |
| DELETE | `/api/items/{id}` | owner | Delete item and its photos |
| POST | `/api/items/{id}/images` | owner | Upload photo (multipart field `file`) |
| DELETE | `/api/items/{id}/images/{imageId}` | owner | Remove photo |
| PUT | `/api/items/{id}/rating` | owner | Set the rating `{ "rating": 1-5 }` |
| DELETE | `/api/items/{id}/rating` | owner | Clear the rating |
| GET | `/api/collections` | yes | Your folders, including the root (the one with no `parentId`) |
| POST | `/api/collections` | yes | Create `{ "name", "visibility", "parentId" }` (parent defaults to root) |
| PUT | `/api/collections/{id}` | owner | Update `{ "name", "visibility", "parentId" }`; moves the folder. The root can't be changed |
| DELETE | `/api/collections/{id}?contents=delete\|move` | owner | Delete a folder. `delete` (default) removes everything inside it; `move` moves the contents up to the parent |
| GET | `/api/tags` | yes | Hashtags used on your items (for the filter) |

## Tests and checks

```sh
cd backend && go vet ./... && go build ./...
cd frontend && npm run build
```
