# Web - AI Assistant Configuration

<!-- Keep commands in sync with root AGENTS.md and Makefile -->

## Overview

Astro static site with Tailwind CSS v4 and Scalar API playground.

## Commands

```bash
make web-dev    # Start Astro dev server
make web-build  # Build Astro static site
make web-lint   # Run astro check (TypeScript diagnostics)
make web-check  # web-lint + web-build
make fmt-web    # Check web formatting (Prettier)
make fmt-web-fix  # Auto-fix web formatting (Prettier)
```

## Coding Conventions

- **Framework:** Astro with static output
- **Styling:** Tailwind CSS v4, CSS-first configuration (no `tailwind.config.js`)
- **Dark mode:** Class strategy (`dark:` variants)
- **API docs:** Scalar API playground at `/docs`
- **No JavaScript frameworks:** Use Astro components and islands only when interactivity is required

## Structure

```
web/
├── src/
│   ├── pages/              # Routes: /, /docs, /tools/*, /imprint, /privacy, /datenschutz
│   ├── layouts/            # BaseLayout with theme system
│   ├── components/         # Shared UI components
│   └── styles/             # Tailwind v4 CSS
├── astro.config.mjs
└── package.json
```
