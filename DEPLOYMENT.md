# Deployment Guide

This guide covers deploying the Traver Hardwood Floors website on a VPS with an existing Caddy reverse proxy.

## Prerequisites

- Docker and Docker Compose installed
- An existing Caddy instance handling HTTPS for your server
- A Postmark account with a server API token
- DNS configured to point `traverhardwoodfloors.com` and `www.traverhardwoodfloors.com` to your server

## Architecture

```
Internet → Caddy (HTTPS) → Docker Container (HTTP:8082)
                                ├── Caddy (static files)
                                └── Go API (contact form)
```

## Deployment Steps

### 1. Clone the Repository

```bash
cd /opt
git clone https://github.com/DukeRupert/traver-hardwood-floors.git
cd traver-hardwood-floors
```

### 2. Configure Environment Variables

```bash
cp .env.example .env
```

Edit `.env` with your values:

```bash
# Port the container listens on (for outer reverse proxy)
LISTEN_PORT=8082

# Postmark configuration
POSTMARK_TOKEN=your-postmark-server-token
FROM_EMAIL=noreply@traverhardwoodfloors.com
TO_EMAIL=chris@traverhardwoodfloors.com

# CORS - must match your public domain
ALLOWED_ORIGIN=https://www.traverhardwoodfloors.com

# Cloudflare Turnstile (spam protection)
TURNSTILE_SECRET=your-turnstile-secret-key
```

### 3. Configure Outer Caddy

Add the following to your main Caddy configuration (usually `/etc/caddy/Caddyfile`):

```caddyfile
www.traverhardwoodfloors.com {
    reverse_proxy localhost:8082
}

traverhardwoodfloors.com {
    redir https://www.traverhardwoodfloors.com{uri} permanent
}
```

Reload Caddy:

```bash
sudo systemctl reload caddy
```

### 4. Build and Start the Container

```bash
docker compose up -d --build
```

### 5. Verify Deployment

Check the container is running:

```bash
docker compose ps
docker compose logs
```

Test the site:

```bash
curl -I https://www.traverhardwoodfloors.com
```

## Updating the Site

To deploy updates:

```bash
cd /opt/traver-hardwood-floors
git pull
docker compose up -d --build
```

## Monitoring

View logs:

```bash
docker compose logs -f
```

Check container health:

```bash
docker compose ps
```

## Troubleshooting

### Container won't start

Check logs for errors:

```bash
docker compose logs
```

### Contact form not working

1. Verify `POSTMARK_TOKEN` is set correctly in `.env`
2. Check that `FROM_EMAIL` domain is verified in Postmark
3. Verify `ALLOWED_ORIGIN` matches your site URL exactly
4. Check container logs for API errors:
   ```bash
   docker compose logs | grep -i error
   ```

### 502 Bad Gateway

1. Ensure the container is running: `docker compose ps`
2. Verify the port matches between `.env` (`LISTEN_PORT`) and Caddy config
3. Check if the container is healthy: `docker compose logs`

### CSS/styling issues

Rebuild the container to regenerate Hugo output:

```bash
docker compose up -d --build --force-recreate
```

## Backup

The site is stateless - all content is in the Git repository. No backup of the container is needed.

To backup environment configuration:

```bash
cp .env .env.backup
```

## Security Notes

- The `.env` file contains secrets - ensure it's not committed to Git
- The container runs as non-root by default (Caddy's default)
- CORS is configured to only allow requests from the specified origin
- The contact form includes Cloudflare Turnstile and honeypot spam protection
