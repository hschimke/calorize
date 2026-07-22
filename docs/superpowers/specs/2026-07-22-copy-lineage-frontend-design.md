# Copy & Lineage Frontend — Design

**Date:** 2026-07-22
**Status:** Approved
**Depends on:** backend copy-lineage feature (`POST /foods/{id}/copy`, `GET /foods/{id}/lineage`, `copied_from_id` on day-copied log entries)

## Goal

Surface the copy-lineage backend in the UI: let users copy foods from the places
they encounter them, explore a food's copy ancestry and descendants, and see
where a day-copied log entry originally came from.

## Deliverables

1. **Copy actions for foods** — on the My Foods list and in food search results.
2. **Lineage modal** — ancestors + copy tree, opened from the foods page.
3. **Log-entry provenance** — a "copied" badge on day-copied entries opening a
   summary modal, backed by one new endpoint (`GET /logs/{id}/lineage`).

Out of scope: lineage statistics/analytics, SVG tree visualization, weight-log
lineage.

## Decisions and rationale

- **Copy surfaces**: My Foods rows (duplicate-to-tweak) and food search results
  (grab a public/system food as an editable copy). Search-result copying is an
  opt-in `showCopy` option on the shared `FoodSearch` component, enabled only in
  the food log's picker — the ingredient picker stays uncluttered.
- **Lineage view is a modal, not a page**: occasional-use, not a destination.
  The tree renders as an indented nested list (`<ul>`) — zero dependencies,
  works at any depth, matches the app's list-based design language. The opened
  food is highlighted; redacted nodes render as "Private food 🔒" and deleted
  nodes are flagged, preserving topology.
- **Log provenance is a summary, not the full chain**: daily "copy yesterday"
  usage builds chains hundreds of entries deep. The modal shows the origin
  entry ("First logged <date> (<meal>)") and the **chain depth** ("Copied
  forward N times since"). Chain depth = copy-steps between this entry and the
  origin (not total descendants of the origin).
- **Pure copy UX**: no confirm dialog, no rename prompt — copying is cheap and
  reversible; a toast confirms and the list refreshes.

## New backend endpoint

`GET /logs/{id}/lineage` → `{ origin: <log entry JSON>, copies: N }`

- Recursive CTE walks `copied_from_id` upward, seeded by `id AND user_id`
  (ownership enforced; foreign/missing entries are 404s). Depth cap 1000
  (log chains grow daily; the foods cap of 64 is too small here).
- Soft-deleted entries stay in the chain; a deleted origin is returned with
  `deleted_at` set and the modal notes "(original entry has been deleted)".
- A non-copy entry returns itself with `copies: 0`.

## Frontend structure

- `js/api.js`: `copyFood(id)`, `getFoodLineage(id)`, `getLogLineage(id)`.
- `js/ui.js`: new `openModal(title)` helper — creates a `<dialog
  class="app-modal">` styled by CSS classes (following the native-dialog
  precedent from the copy-day dialog, not the legacy inline-styled overlays),
  returns `{ body, close }`, closes on ×/backdrop click.
- `js/food-ui.js`: Copy + Lineage buttons per My Foods row; lineage modal
  rendering (ancestors block, recursive tree, per-node Copy action, empty
  state). Migrates the recipe badge's inline style to a shared `.badge` class.
- `js/food-search.js`: `showCopy` constructor option adding a copy button per
  result row (with `stopPropagation` — the whole row is clickable).
- `js/foodlog.js`: "copied" badge in `buildLogRow` for entries with
  `copied_from_id`; click opens the summary modal.
- `css/main.css`: new section for `.badge`/`.badge-copied`, `.app-modal`
  (header, close, scrollable body), and `.lineage-tree` (nested indentation
  with border-left connectors, `.lineage-current` highlight).

## Error handling

All fetch failures surface via `showToast(..., 'error')`; modals only open
after their data has loaded.

## Testing

- Go unit tests for the chain-summary walk (chain depth, ownership, deleted
  origin, non-copy entry).
- Integration coverage in `tests/11_copy_lineage.sh` for the new endpoint.
- Frontend verified manually in the browser (no JS test harness): copy flows
  from both surfaces, lineage modal on a multi-branch tree, badge + summary
  modal after a day copy, mobile-width layout.
