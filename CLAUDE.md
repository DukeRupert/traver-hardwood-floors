# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Traver Hardwood Floors — a brochure website for a hardwood flooring business in Helena/Bozeman, Montana. Built with Hugo (static site generator) and a Go API backend for form handling, deployed as a single Docker container.

## Architecture

```
Internet → Outer Caddy (HTTPS/TLS on VPS) → Docker Container (HTTP:8082)
                                                ├── Inner Caddy (static files + reverse proxy)
                                                └── Go API (:8080, contact/estimate forms)
```

- **Hugo** generates static HTML into `public/` (built inside Docker via `hugomods/hugo:exts`)
- **Go API** (`api/`) handles form submissions, sends email via Postmark, validates with Cloudflare Turnstile
- **Inner Caddy** serves static files from `/srv` and proxies `/api/*` to the Go backend
- **Alpine.js** used for interactive UI components (FAQ accordion, mobile menu)
- **Plausible Analytics** for privacy-respecting analytics

## Development Commands

```bash
# Local Hugo dev server (live reload)
hugo server

# Build static site
hugo --gc --minify

# Build Go API
cd api && go build -o contact-api .

# Run Go API locally (needs env vars)
cd api && POSTMARK_TOKEN=test ./contact-api

# Docker build and run
docker compose up -d --build

# View container logs
docker compose logs -f
```

## Deployment

Pushes to `master` trigger GitHub Actions → Docker build → push to Docker Hub → SSH deploy to VPS. See `.github/workflows/deploy.yml`.

## Key Files

- `hugo.toml` — site config, params (phone, social links, Turnstile key), menu structure
- `assets/css/main.css` — all styles, uses CSS custom properties (no Tailwind)
- `layouts/_default/baseof.html` — base template with SEO meta, schema.org markup, analytics
- `layouts/partials/` — shared components (header, footer, CTA section, FAQ accordion)
- `content/` — markdown pages with front matter (services, projects, landing pages)
- `data/faq.yaml` — FAQ content data
- `api/main.go` — Go API server (contact form, estimate form with file uploads)
- `Caddyfile` — inner Caddy config (static serving + API reverse proxy, 12MB upload limit)
- `brand-guide.md` — brand voice, colors, typography guidelines

## Styling

Custom CSS with CSS variables — no utility framework. Color palette named "Aged Fir at Dusk" — dark espresso/walnut tones with amber accent. Fonts: Playfair Display (headings, serif, weight 700) and Source Sans 3 (body, sans-serif) loaded from Google Fonts. See `traver-design-system.html` for the complete visual reference and `traver-hardwood-brand-guide.md` for brand rules.

## Content Pages

Hugo content uses front matter for page-specific data (testimonials, service details, SEO titles). Landing pages (`bozeman.md`, `refinishing.md`) target specific geographic/service keywords. Project case studies are in `content/projects/`.

## Environment Variables (for API)

- `POSTMARK_TOKEN` — email service API key (required)
- `TURNSTILE_SECRET` — Cloudflare spam protection secret
- `ALLOWED_ORIGIN` — CORS origin (must match production domain exactly)
- `FROM_EMAIL` / `TO_EMAIL` — email sender/recipient

## Design Context

### Users
Homeowners investing in quality hardwood flooring — custom home builds, historic home restorations, and renovations in Helena, Bozeman, and Butte, Montana. Mid-to-high budget ($5K–$30K+ projects). They're browsing contractors, comparing options, and looking for someone they can trust with a significant investment in their home.

### Brand Personality
Craftsman, rooted, trustworthy. A 6th-generation Montana native with 18 years of hardwood expertise. The voice is a skilled tradesman talking to a homeowner in their living room — knowledgeable but not condescending, specific but not jargon-heavy. Never corporate, franchise-feel, salesy, or template-generic.

### Emotional Goal
**Trust and confidence.** Visitors should feel safe choosing Traver — this is clearly a skilled, established craftsman who stands behind his work. The site earns trust through real project photography, specific details, named awards, and a visible family story.

### Aesthetic Direction
Warm, craft-forward, photo-first. The palette named "Aged Fir at Dusk" is drawn from the floors themselves — aged fir, white oak, walnut endgrain lit by golden-hour light. Dark navigation and dark sections create contrast and authority. Never corporate, never generic, never sterile.

**Color palette** (see `traver-design-system.html` for full swatches):
- Backgrounds: `--bg: #FAF7F2` (page bg), `--surface: #F0EBE3` (alt sections), `--border: #DDD6CC`
- Dark tones: `--bg-mid: #2E2318` (nav, portfolio, testimonials, footer), `--bg-dark: #1C1710` (proof bar, hero overlays)
- Text: `--text: #1C1710` (primary), `--muted: #8C7E6E` (body copy, captions), `--text-inv: #FAF7F2` (on dark)
- Accent: `--accent: #C8882A` (CTAs, eyebrows, proof numbers, CTA strip bg), `--accent-light: #E8AA52` (hover states, phone number on dark bg)
- No cool blues, no purples, no neons. No sage green. The palette stays in the warm brown/tan/amber family.

**Typography:**
- Display/headings: Playfair Display (serif), weight **700** (bold). Hero headline: `clamp(52px, 7vw, 88px)`. Section headlines: `clamp(32px, 4vw, 48px)`.
- Body/UI: Source Sans 3 (sans-serif). Body at 17px/1.75 line-height, color `--muted`.
- Eyebrow/label: Source Sans 3 at 11px, weight 600, uppercase, `letter-spacing: 0.18em`, color `--accent`.
- **Never use Inter, Roboto, or system sans-serif fonts.** Two fonts only: Playfair Display + Source Sans 3.

**Component style:**
- Buttons: **square** (no border-radius), uppercase text with letter-spacing, 13px font-size. Primary = Accent bg / white text. Ghost = transparent + white border (on dark). White = white bg / accent text (on CTA strip).
- Service cards: Numbered (01-04), `--border` top accent, hover changes to `--accent` border.
- Testimonials: Dark bg (`--bg-mid`), large amber quote marks, Playfair italic white text.
- Nav: Dark bg (`--bg-mid`), text-based logo with "Helena, Montana" tagline, phone number in `--accent-light`, "Get Estimate" CTA in `--accent`.
- Footer: Dark bg (`--bg-dark`). Four-column layout (brand, services, company, contact).
- CTA strip: `--accent` background, white buttons. Headline left, buttons right.

**Layout:**
- Content container: `max-width: 1200px` — wider, photo-forward
- Section vertical padding: 88–96px
- Section backgrounds alternate between `--bg` and `--surface`, with `--bg-mid` and `--bg-dark` for dark sections
- Hero: Full-bleed, bottom-left aligned content (not centered), gradient overlay
- Proof bar: 4-stat dark strip below hero with large Playfair numbers in accent
- Photography and hero sections break to full bleed

Photos are real project work — no stock photography ever. Images dominate over text. See `traver-design-system.html` for complete component patterns, `traver-copy-package.html` for all copy, and `traver-hardwood-brand-guide.md` for voice specifications.

### Accessibility
WCAG AA compliance. Ensure sufficient contrast ratios, keyboard navigation, descriptive alt text on all images, proper focus indicators using accent ring color (`box-shadow: 0 0 0 3px rgba(200, 136, 42, 0.35)` on focus), and aria-labels on decorative elements like star ratings.

### Design Principles
1. **Photos first, text second** — this is a craft portfolio site. Images should always be larger than text. The wood IS the content.
2. **Warm everything** — alternate between `--bg` and `--surface` for light sections, use `--bg-mid` and `--bg-dark` for dark sections. Never use pure white or cool grays.
3. **Specific over generic** — "reclaimed Australian Karri wood" not "exotic hardwood." Real award names, real city names, real phone numbers visible everywhere.
4. **One clear action** — maximum one primary CTA per section. "Request an Estimate" is always the primary conversion action; phone (with full number) is secondary.
5. **Earn trust, don't claim it** — show the work, name the awards, quote real clients with their city. Never say "the best" without a verifiable source.
6. **Dark contrast** — the dark nav, proof bar, portfolio, and testimonial sections create rhythm and authority. The amber accent on dark backgrounds is the site's signature visual.
