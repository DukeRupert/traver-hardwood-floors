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

Custom CSS with CSS variables — no utility framework. Color palette is wood-inspired warm tones with walnut/sand/grain primaries and slate/sage secondaries. Fonts: Playfair Display (headings, serif) and Source Sans 3 (body, sans-serif) loaded from Google Fonts. See `ui-guide.html` for the complete visual reference including color swatches, typography specimens, and component patterns.

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
Warm, editorial, craft-forward. The visual language draws from the logo's wood tones — walnut brown, milled-grain sand, parchment paper. The feel should be a well-kept workshop: warm, tactile, grounded. Never corporate, never generic, never sterile.

**Color palette** (see `ui-guide.html` for full swatches):
- Primary wood tones: Walnut `#4A2E14` (brand anchor, headings, primary CTAs), Oak `#7C4F28` (hover states, borders, links), Sand `#C8A97E` (accents, eyebrows, borders), Grain `#E8D5BC` (subtle fills)
- Warm neutral backgrounds: Parchment `#F5EEE4` (alt sections), Cream `#FAF6F1` (default sections), Warm White `#FDFCFA` (page bg, cards). **Never use pure white `#FFFFFF` as a background.**
- Secondary accents: Slate `#2C3A47` (footer, dark sections), Sage `#6B7C5A` (phone CTA, location tags), Sage Pale `#E8EEE3` (location badge bg)
- Text: Primary `#1E1A16`, Secondary `#5A4E43`, Muted `#9A8E83`
- No cool blues, no purples, no neons. The palette stays in the warm brown/tan/sand family with slate and sage as the only cool tones.

**Typography:**
- Display/headings: Playfair Display (serif), weight 400 (normal), in Walnut `#4A2E14`. All heading levels use Playfair — this is the brand's editorial warmth.
- Body/UI: Source Sans 3 (sans-serif). Body at 16px/1.75 line-height. Subheads at 18px weight 300 (light).
- Eyebrow/label: Source Sans 3 at 11px, weight 600, uppercase, `letter-spacing: 0.12em`, in Sand `#C8A97E`.
- **Never use Inter, Roboto, or system sans-serif fonts.** Two fonts only: Playfair Display + Source Sans 3.

**Component style:**
- Buttons: sharp corners (`border-radius: 2px`), uppercase text with letter-spacing, 14px font-size. Primary = Walnut bg / Grain text. Phone CTA = Sage bg / white text.
- Cards: subtle `border-light` with Sand top-accent border on service cards. Minimal shadows. Cream/warm-white backgrounds.
- Testimonials: Parchment bg, left Sand border accent, quote in Playfair italic.
- Nav: Warm White bg with Sand-tinted bottom border, sticky on scroll. Phone number always visible on desktop.
- Footer: Slate `#2C3A47` background (not Walnut). Three-column layout.

**Layout:**
- Content container: `max-w-4xl` (896px) — narrower, editorial feel
- Body copy max-width: 640px for readability
- Section vertical padding: minimum 64px top and bottom
- Section backgrounds alternate between Cream and Parchment
- Photography and hero sections can break to full bleed

Photos are real project work — no stock photography ever. Images dominate over text. See `ui-guide.html` for complete component patterns and `brand-guide.md` for voice specifications.

### Accessibility
WCAG AA compliance. Ensure sufficient contrast ratios, keyboard navigation, descriptive alt text on all images, proper focus indicators using Sand ring color (`box-shadow: 0 0 0 3px rgba(200,169,126,0.2)` on focus), and aria-labels on decorative elements like star ratings.

### Design Principles
1. **Photos first, text second** — this is a craft portfolio site. Images should always be larger than text. The wood IS the content.
2. **Warm everything** — alternate between Cream `#FAF6F1` and Parchment `#F5EEE4` for section backgrounds. Never use pure white or cool grays. Every surface should feel warm.
3. **Specific over generic** — "reclaimed Australian Karri wood" not "exotic hardwood." Real award names, real city names, real phone numbers visible everywhere.
4. **One clear action** — maximum one primary CTA per section. "Request an Estimate" is always the primary conversion action; phone (with full number) is secondary in Sage green.
5. **Earn trust, don't claim it** — show the work, name the awards, quote real clients with their city. Never say "the best" without a verifiable source.
6. **Editorial, not template** — Playfair Display headings, narrow content column, generous whitespace, sharp button corners. The site should feel designed, not assembled from a UI kit.
