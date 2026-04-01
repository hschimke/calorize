# Future Improvements

This document tracks architectural improvements and technical debt identified during audits.

## Backend
- [ ] **Hydrate Recipe Ingredients**: Update `GET /foods/{id}` to return hydrated ingredient details (names, macros) to avoid N+1 requests on the frontend (currently handled in `food-ui.js`).

## Frontend
- [ ] **Robust Date Handling**: Replace "local noon" heuristic in `foodlog.js` with a more robust library or explicit offset handling to prevent day-boundary shifts in extreme timezones.
- [ ] **Cookie SameSite**: Consider switching `session_id` cookie from `SameSite=Strict` to `SameSite=Lax` in `internal/auth/auth.go` to improve UX for users arriving via external links.
