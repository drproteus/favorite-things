# Deploying Favorite Things with Ansible

This playbook deploys Favorite Things to a production host as:

- a static Go binary at `/opt/favorite-things/favorite-things-api`
- a hardened `favorite-things.service` systemd unit
- a local PostgreSQL database
- a local MinIO object storage server
- nginx terminating TLS with a Let's Encrypt certificate and proxying to the API

The SSH connection comes from your existing `~/.ssh/config` entry for the target;
no user or key is stored in the inventory.

## Assumptions

- The target is Debian or Ubuntu.
- The target has SSH access via your `~/.ssh/config` entry for `things.skinet.org`, with sudo access.
- Ports 80/443 are reachable from the internet.
- DNS for `things.skinet.org` points at the target.
- The controller has Ansible and Go installed. The playbook cross-compiles the
  binary locally for the target's architecture (`amd64` or `arm64`).

## First-time setup

Install the PostgreSQL collection used by the playbook:

```bash
cd deploy
ansible-galaxy collection install -r requirements.yml
```

Create the secrets file:

```bash
cp group_vars/all/vault.yml.example group_vars/all/vault.yml
# edit it with your secrets (GitHub/Google OAuth, MinIO password, Gemini key), then encrypt it:
ansible-vault encrypt group_vars/all/vault.yml
```

Fill in `vault.yml`:

- `favorite_things_db_password` — a URL-safe random string such as
  `openssl rand -hex 24`.
- `favorite_things_minio_root_password` — a URL-safe random string.
- `favorite_things_letsencrypt_email` — email for Let's Encrypt account.
- `favorite_things_gemini_api_key` — your Google Gemini API key (optional if not using photo suggestions).
- At least one OAuth pair: GitHub or Google (required for authentication).

Register the OAuth callback URLs with the providers:

- GitHub: `https://things.skinet.org/api/auth/github/callback`
- Google: `https://things.skinet.org/api/auth/google/callback`

## Deploy

From this directory, run the playbook with the inventory file:

```bash
cd deploy
ansible-playbook site.yml -i hosts.ini --ask-vault-pass --ask-become-pass
```

The playbook reads the target from `hosts.ini` (which is `things.skinet.org`) and uses your SSH config for authentication.

On the first run the playbook:
1. Provisions PostgreSQL and MinIO
2. Builds and installs the API binary
3. Serves the site over HTTP for the ACME challenge
4. Obtains the TLS certificate
5. Installs the HTTPS nginx configuration

Later runs skip certificate issuance and use the existing certificate.

## Updating

Re-run the playbook. It rebuilds the binary, replaces it if the checksum changed,
restarts the service, and reloads nginx when configuration changes:

```bash
cd deploy
ansible-playbook site.yml -i hosts.ini --ask-vault-pass
```

## Checking the deployment

```bash
curl -fsS https://things.skinet.org/api/health
ssh things.skinet.org 'systemctl status favorite-things --no-pager'
ssh things.skinet.org 'journalctl -u favorite-things -n 50 --no-pager'
ssh things.skinet.org 'nginx -t'
```

## Frontend deployment

The Vite-built frontend is static HTML/JS. Deploy it via:

1. **S3-compatible storage** (MinIO): Upload `frontend/dist/` to an S3 bucket and configure nginx to proxy `/` to it, or serve directly from MinIO.
2. **CDN**: Push the dist folder to a CDN for geographic distribution.
3. **Same server**: Copy `frontend/dist/` to `/var/www/html/favorite-things/` and configure nginx to serve it.

This playbook does not manage the frontend. Adjust nginx configuration (`templates/nginx-site.conf.j2`) as needed.

## Notes

- Secrets live in `group_vars/all/vault.yml`, which is git-ignored. Never commit
  it; if encrypted, keep the vault password outside the repository.
- The API binds only to loopback (`127.0.0.1:8080`). nginx is the public listener.
- The playbook assumes the stock Debian/Ubuntu `pg_hba.conf` allows password
  authentication from `127.0.0.1`.
- MinIO runs on `127.0.0.1:9000` (S3 API) and `127.0.0.1:9001` (web UI); nginx
  does not expose them publicly.
- Certbot's systemd timer handles renewal, and a deploy hook reloads nginx after
  a renewal.
- The playbook does not manage a firewall. Ensure SSH, HTTP and HTTPS are allowed
  on the host.
- `site.yml` fails early with a readable message if a required secret is missing
  or the target is not Debian/Ubuntu.

## Environment variables reference

The API reads these from `favorite-things.env`:

```
DATABASE_URL              PostgreSQL connection string (built from secrets)
MINIO_URL                 MinIO S3 endpoint (http://127.0.0.1:9000)
MINIO_ACCESS_KEY          MinIO root access key
MINIO_SECRET_KEY          MinIO root secret key (from vault)
MINIO_BUCKET              S3 bucket for photos (auto-created)
GEMINI_API_KEY            Google Gemini API key (optional, for photo suggestions)
SUGGEST_ENABLED           "true" to enable photo suggestions (requires GEMINI_API_KEY)
GITHUB_CLIENT_ID          GitHub OAuth client ID
GITHUB_CLIENT_SECRET      GitHub OAuth client secret
GOOGLE_CLIENT_ID          Google OAuth client ID
GOOGLE_CLIENT_SECRET      Google OAuth client secret
PUBLIC_URL                Public HTTPS URL for the API (https://<domain>/api)
```
