# Deploying Favorite Things with Ansible

This playbook deploys Favorite Things to a production host as:

- a static Go binary at `/opt/favorite-things/favorite-things-api`
- a hardened `favorite-things.service` systemd unit
- a local PostgreSQL database
- AWS S3 for photo storage
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
- You have AWS S3 credentials and a bucket already created.

## First-time setup

Install the PostgreSQL collection used by the playbook:

```bash
cd deploy
ansible-galaxy collection install -r requirements.yml
```

Create the secrets file:

```bash
cp group_vars/all/vault.yml.example group_vars/all/vault.yml
# edit it with your secrets (AWS S3, GitHub/Google OAuth, Gemini key), then encrypt it:
ansible-vault encrypt group_vars/all/vault.yml
```

Fill in `vault.yml`:

- `favorite_things_db_password` — a URL-safe random string such as
  `openssl rand -hex 24`.
- `favorite_things_aws_access_key_id` — your AWS IAM user's access key.
- `favorite_things_aws_secret_access_key` — your AWS IAM user's secret key.
- `favorite_things_aws_s3_bucket` — name of your S3 bucket for photos.
- `favorite_things_aws_s3_region` — AWS region where the bucket lives (e.g. `us-east-1`).
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
1. Provisions PostgreSQL
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

1. **S3**: Upload `frontend/dist/` to your S3 bucket and configure nginx to proxy or serve it.
2. **CDN**: Push the dist folder to a CDN for geographic distribution.
3. **Same server**: Copy `frontend/dist/` to `/var/www/html/favorite-things/` and configure nginx to serve it.

This playbook does not manage the frontend. Adjust nginx configuration (`templates/nginx-site.conf.j2`) as needed.

## Notes

- Secrets live in `group_vars/all/vault.yml`, which is git-ignored. Never commit
  it; if encrypted, keep the vault password outside the repository.
- The API binds only to loopback (`127.0.0.1:8080`). nginx is the public listener.
- The playbook assumes the stock Debian/Ubuntu `pg_hba.conf` allows password
  authentication from `127.0.0.1`.
- AWS S3 credentials are passed via environment variables to the API.
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
S3_ENDPOINT               Regional S3 endpoint (https://s3.<region>.amazonaws.com)
S3_BUCKET                 S3 bucket name for photos (must already exist)
S3_REGION                 AWS region (e.g. us-east-1)
S3_PATH_STYLE             "false" for AWS S3
AWS_ACCESS_KEY_ID         AWS IAM user access key
AWS_SECRET_ACCESS_KEY     AWS IAM user secret key
ADDR                      Listen address (127.0.0.1:8080)
PUBLIC_URL                Public HTTPS origin (https://things.skinet.org)
FRONTEND_URL              Post-login redirect origin (https://things.skinet.org)
COOKIE_SECURE             "true" because the site is served over HTTPS
GEMINI_API_KEY            Google Gemini API key (optional, for photo suggestions)
SUGGEST_ENABLED           "true" to enable photo suggestions (requires GEMINI_API_KEY)
GITHUB_CLIENT_ID          GitHub OAuth client ID
GITHUB_CLIENT_SECRET      GitHub OAuth client secret
GOOGLE_CLIENT_ID          Google OAuth client ID
GOOGLE_CLIENT_SECRET      Google OAuth client secret
```

The API reads the standard AWS access key variables, matching local development.
Local MinIO defaults only apply when those credentials are unset.
