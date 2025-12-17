# Contact Form API

A simple Go server that handles contact form submissions and sends emails via Postmark.

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `POSTMARK_TOKEN` | Yes | - | Your Postmark server API token |
| `FROM_EMAIL` | No | `noreply@traverhardwoodfloors.com` | Sender email address |
| `TO_EMAIL` | No | `chris@traverhardwoodfloors.com` | Recipient email address |
| `ALLOWED_ORIGIN` | No | `https://www.traverhardwoodfloors.com` | CORS allowed origin |
| `PORT` | No | `8080` | Port to run the server on |

## Building

```bash
cd api
go build -o contact-api .
```

## Running

```bash
export POSTMARK_TOKEN="your-postmark-token"
./contact-api
```

## Endpoints

### POST /api/contact

Submit a contact form.

**Request Body:**
```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "phone": "406-555-1234",
  "message": "I'm interested in getting a quote for hardwood floor installation..."
}
```

**Response (Success):**
```json
{
  "status": "success",
  "message": "Thank you for your message. We'll be in touch soon!"
}
```

### GET /health

Health check endpoint. Returns `OK` with status 200.

## Deployment with systemd

Create `/etc/systemd/system/traver-contact-api.service`:

```ini
[Unit]
Description=Traver Hardwood Floors Contact API
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/traver-contact-api
ExecStart=/opt/traver-contact-api/contact-api
Restart=always
RestartSec=5
Environment=POSTMARK_TOKEN=your-token-here
Environment=FROM_EMAIL=noreply@traverhardwoodfloors.com
Environment=TO_EMAIL=chris@traverhardwoodfloors.com
Environment=ALLOWED_ORIGIN=https://www.traverhardwoodfloors.com
Environment=PORT=8080

[Install]
WantedBy=multi-user.target
```

Then:
```bash
sudo systemctl daemon-reload
sudo systemctl enable traver-contact-api
sudo systemctl start traver-contact-api
```

## Nginx Proxy

Add to your Nginx config to proxy `/api` requests:

```nginx
location /api/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```
