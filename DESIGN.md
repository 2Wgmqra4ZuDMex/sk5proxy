# SK5 Proxy Console Design System

## 0. Research Log

- Embedded refs: shortlisted `vercel`, `raycast`, `warp` → picked `taste-skill.md` (operational default) + `vercel.md` (developer tool precision, shadow-as-border, monochrome + functional accent) because this is a small admin console for developers, not a marketing site.
- Lazyweb: skipped — proportionate for a tiny operational console; no real-product screens needed beyond the Vercel devtool reference.
- Imagen drafts: skipped — no visual mockup needed for a functional admin UI; the Vercel reference provides sufficient token grounding.
- Skipped lanes: image/lazyweb/imagen — irrelevant for a dependency-free embedded console with no brand imagery.

## 1. Atmosphere & Identity

A quiet command surface. Monochrome precision with functional color for connection state. The signature is shadow-as-border — surfaces separated by 1px shadow rings rather than hard borders, creating depth without weight. Active state is unmistakable: a filled accent pill and bold label. The console feels like a developer tool: dense when needed, spacious when not, with every element earning its pixel.

## 2. Color

### Palette

| Role | Token | Light | Dark | Usage |
|------|-------|-------|------|-------|
| Surface/primary | --surface-primary | #ffffff | #0a0a0a | Page background |
| Surface/secondary | --surface-secondary | #fafafa | #141414 | Card background |
| Surface/elevated | --surface-elevated | #ffffff | #1a1a1a | Modal, dropdown |
| Text/primary | --text-primary | #171717 | #fafafa | Headlines, body |
| Text/secondary | --text-secondary | #666666 | #a0a0a0 | Captions, hints |
| Text/tertiary | --text-tertiary | #707070 | #8a8a8a | Disabled, muted (WCAG AA ≥4.5:1) |
| Border/default | --border-default | rgba(0,0,0,0.08) | rgba(255,255,255,0.08) | Shadow-as-border |
| Border/subtle | --border-subtle | #ebebeb | #2a2a2a | Soft separations |
| Accent/primary | --accent-primary | #0a72ef | #3b8bff | Active state, links, focus |
| Accent/hover | --accent-hover | #0058cc | #60a5fa | Hover state |
| Accent/filled | --accent-filled | #0a72ef | #2563eb | White-text filled controls (badge, btn-primary); ≥4.5:1 in both modes |
| Accent/filled hover | --accent-filled-hover | #0058cc | #1d4ed8 | Hover for filled controls |
| Status/success | --status-success | #16a34a | #22c55e | Connection active |
| Status/warning | --status-warning | #d97706 | #f59e0b | Cautions |
| Status/error | --status-error | #dc2626 | #ef4444 | Errors, destructive |
| Status/error hover | --status-error-hover | #b91c1c | #dc2626 | Destructive hover state |
| Status/info | --status-info | #0a72ef | #3b8bff | Informational |

### Rules
- Shadow-as-border: `box-shadow: 0 0 0 1px var(--border-default)` replaces CSS borders on cards, inputs, buttons.
- Accent used ONLY for interactive elements and active state. Never decorative.
- Never introduce a color not in this table. Extend the table first.
- Dark mode component contrast: secondary button uses `--surface-secondary` background (not `--surface-primary`) to distinguish from card surface; type badge adds a `rgba(255,255,255,0.12)` ring in dark mode since `--surface-secondary` vs `--surface-primary` delta is too subtle (~1.1:1) for reliable badge recognition.
- Dark mode filled controls (WCAG AA ≥4.5:1 with white text): `.btn-primary` and `.badge-active` use `var(--accent-filled)` (`#2563eb` in dark mode, not `--accent-primary` `#3b8bff` which is ~3.98:1); `.btn-destructive` uses `#dc2626` (not `--status-error` `#ef4444` which is ~3.98:1). Hover states use `var(--accent-filled-hover)` (`#1d4ed8`) and `#b91c1c` respectively.
- Error banners (`.form-error`, `.action-error`, `.confirm-error`): white text (`#ffffff`) on solid `var(--status-error)` background. Light mode `#dc2626` → 4.79:1 ✓. Dark mode overrides to `#dc2626` (not `#ef4444`) → 4.79:1 ✓. Never use `color-mix` 8% tint for error banners (~4.27:1 fails AA).

## 3. Typography

### Scale

| Level | Size | Weight | Line Height | Tracking | Usage |
|-------|------|--------|-------------|----------|-------|
| H1 | 24px / 1.5rem | 600 | 1.3 | -0.5px | Page title |
| H2 | 18px / 1.125rem | 600 | 1.4 | -0.3px | Section headers |
| H3 | 16px / 1rem | 600 | 1.5 | -0.2px | Card titles |
| Body/lg | 16px / 1rem | 400 | 1.6 | 0 | Lead paragraphs |
| Body | 14px / 0.875rem | 400 | 1.5 | 0 | Default text |
| Body/sm | 13px / 0.8125rem | 400 | 1.4 | 0 | Secondary info |
| Caption | 12px / 0.75rem | 500 | 1.3 | 0.02em | Labels, metadata |

### Font Stack
- Primary: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif
- Mono: ui-monospace, SFMono-Regular, "Roboto Mono", Menlo, Monaco, "Courier New", monospace

### Rules
- System font stack for fast load, no external fonts.
- Body text never below 13px.
- Max 2 font families (sans + mono).

## 4. Spacing & Layout

### Base Unit
All spacing derives from a base of **4px**.

| Token | Value | Usage |
|-------|-------|-------|
| --space-1 | 4px | Tight: icon-to-label |
| --space-2 | 8px | Compact: list items, inline groups |
| --space-3 | 12px | Default: form field padding |
| --space-4 | 16px | Standard: card padding |
| --space-5 | 20px | Comfortable: section inner spacing |
| --space-6 | 24px | Generous: card padding (default) |
| --space-8 | 32px | Separated: between card groups |

### Grid
- Max content width: 960px (centered)
- Breakpoints: sm 640px, md 768px, lg 1024px

### Rules
- Tokenize design intent. Browser mechanics (auto, %, clamp) stay raw.
- Single column layout for this console; no complex grids needed.

## 5. Components

### Button
- **Structure**: `<button class="btn btn-primary">` or `<button class="btn btn-secondary">`
- **Variants**: primary (filled accent), secondary (shadow-bordered; dark mode: `--surface-secondary` bg for card contrast), destructive (red)
- **Spacing**: padding 8px 16px, gap 8px for icon+label
- **States**: default, hover (slight darken), active (translate -1px), focus (2px accent outline), disabled (opacity 0.5)
- **Accessibility**: keyboard reachable, visible focus, aria-label for icon-only
- **Motion**: 150ms ease-out on transform/opacity only; state/color changes are instant
- **Layout**: inline-flex, align-center

### Card
- **Structure**: `<div class="card">` with optional `<header>`, `<body>`, `<footer>`
- **Variants**: default (shadow-bordered), active (inset accent left edge via box-shadow + filled background tint)
- **Spacing**: padding 16px, gap 12px between sections
- **States**: default, hover (subtle shadow intensify)
- **Accessibility**: semantic HTML, focusable children
- **Motion**: 200ms ease on shadow
- **Layout**: block, full-width

### Input
- **Structure**: `<label class="input-label">` + `<input class="input">`
- **Variants**: text, password, select
- **Spacing**: padding 8px 12px, label gap 4px
- **States**: default, focus (accent ring), error (red ring + message), disabled
- **Accessibility**: label always visible, error linked via aria-describedby
- **Motion**: 150ms ease on focus ring
- **Layout**: block, full-width

### SectionHeader
- **Structure**: `<div class="section-header">` with `<h2>` title and add button
- **Variants**: default
- **Spacing**: margin-bottom 24px, gap 12px between title and button
- **States**: static
- **Accessibility**: semantic heading level
- **Motion**: none
- **Layout**: flex row, space-between

### UpstreamRow
- **Structure**: card with name, type badge, address, username, actions (edit/delete)
- **Variants**: default (no active variant; legacy activeId is not routing)
- **Spacing**: padding 16px, gap 12px between elements
- **States**: default, hover
- **Accessibility**: keyboard actions, confirm delete, aria-live for status
- **Motion**: 200ms ease on hover shadow
- **Layout**: flex row on desktop, column on mobile

### ListenerRow
- **Structure**: card with name, type badge, address, current upstream name, enabled badge, switch control (select + apply button), actions (toggle/edit/delete)
- **Variants**: default, enabled (subtle accent left edge via inset box-shadow), disabled (muted opacity)
- **Spacing**: padding 16px, gap 12px between elements
- **States**: default, enabled, disabled, hover, switching (spinner on apply)
- **Accessibility**: keyboard actions, confirm delete, aria-live for status, upstream select has accessible label
- **Motion**: 200ms ease on state change
- **Layout**: flex row on desktop, column on mobile; switch control wraps on narrow screens

### ListenerDialog
- **Structure**: `<dialog>` with form fields: name, type (socks5/http), address, upstream (dropdown), enabled (checkbox)
- **Variants**: add (ID field editable), edit (ID field read-only)
- **Spacing**: padding 24px, gap 16px between fields
- **States**: open, closed, loading (submit)
- **Accessibility**: focus trap, escape to close, aria-modal, checkbox has visible label
- **Motion**: 200ms ease-in-out fade + scale
- **Layout**: centered via `margin: auto`, max-width 480px

### Modal
- **Structure**: `<dialog class="modal">` with `<form>` inside
- **Variants**: confirm (destructive action), form (edit/add)
- **Spacing**: padding 24px, gap 16px
- **States**: open, closed, loading (submit)
- **Accessibility**: focus trap, escape to close, aria-modal; confirm dialog contains `#confirm-error` region with `role="alert"` for in-dialog delete failure display (cleared on open/close/retry)
- **Motion**: 200ms ease-in-out fade + scale
- **Layout**: centered via `margin: auto` (overrides global `* { margin: 0 }` reset), max-width 480px, viewport-contained

### StatusBadge
- **Structure**: `<span class="badge badge-success">`
- **Variants**: success (green), warning (yellow), error (red), info (blue), type (neutral; dark mode: subtle white ring for surface distinction)
- **Spacing**: padding 2px 8px, radius 9999px
- **States**: static
- **Accessibility**: color + text (not color alone)
- **Motion**: none
- **Layout**: inline-flex

### EmptyState
- **Structure**: centered div with icon, title, description, action
- **Variants**: no-upstreams, error, loading
- **Spacing**: padding 48px, gap 16px
- **States**: static
- **Accessibility**: semantic heading, actionable
- **Motion**: none
- **Layout**: flex column, center

## 6. Motion & Interaction

### Timing

| Type | Duration | Easing | Usage |
|------|----------|--------|-------|
| Micro | 100-150ms | ease-out | Button press, toggle |
| Standard | 200ms | ease-in-out | Modal open, state change |
| Emphasis | 300ms | cubic-bezier(0.16, 1, 0.3, 1) | Page load |

### Rules
- Only animate `transform` and `opacity`. Never animate layout properties.
- Every interactive element has hover + active + focus states.
- Reduced motion: respect `prefers-reduced-motion` — disable non-essential animation.

## 7. Depth & Surface

### Strategy
Shadow-as-border (Vercel-inspired). No traditional CSS borders on cards/inputs/buttons.

| Level | Value | Usage |
|-------|-------|-------|
| Ring | `0 0 0 1px var(--border-default)` | Cards, inputs, buttons |
| Subtle | Ring + `0 2px 4px rgba(0,0,0,0.04)` | Elevated cards |
| Focus | `0 0 0 2px var(--accent-primary)` | Keyboard focus |

## 8. Accessibility Constraints & Accepted Debt

### Constraints
- WCAG 2.1 AA — contrast floor 4.5:1 body / 3:1 large text
- Visible focus on every interactive element
- Full keyboard reachability (Tab, Enter, Escape)
- `prefers-reduced-motion` respected
- Chinese UI text, semantic HTML, aria-labels for icons
- Confirm destructive actions (delete); upstream name wrapped in Chinese curly quotes (\u201c \u201d)
- Status announcements via aria-live
- Backend error messages mapped client-side to concise Chinese; unknown errors fall back to "操作失败，请检查配置" — never expose raw English internals to users
- Network/fetch failures caught and mapped to stable Chinese message "网络连接失败，请检查网络" — never expose raw browser error text
- Delete failures displayed inside confirm dialog via `#confirm-error` role=alert region (not behind modal in `#action-error`); cleared on open/close/retry
- Delete confirmation message built with separate name node (`.confirm-name`) for independent wrapping, plus U+2060 WORD JOINER between 恢 and 复 to prevent phrase split across lines; CSS `word-break: auto-phrase` on `.confirm-message` for phrase-aware breaking where supported. Exact spoken text preserved: 确定删除"NAME"？删除后无法恢复。

### Accepted Debt
| Item | Location | Why accepted | Owner / Exit |
|------|----------|--------------|--------------|
| No i18n framework | index.html | Tiny console, Chinese-only scope | Add if multi-language needed |
| System fonts only | style.css | No CDN/build constraint | Add custom fonts if brand needed |
