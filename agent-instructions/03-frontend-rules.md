# Frontend Rules

## Preserve existing UI

The current React UI is the baseline. Keep the existing workbench, upload interaction, prompt editor, model/parameter selectors, result canvas, history panel concept, modals, Tailwind styles, and tests unless a change is necessary for backend integration.

## Required frontend migration direction

- Replace frontend Provider Adapters with backend API calls.
- Replace local history as the primary data source with project assets and task history APIs.
- Replace local API key settings with Provider management screens backed by backend APIs.
- Replace local generation status with SSE task events.
- Keep local state only for drafts, transient previews, and compatibility helpers.

## Forbidden frontend patterns

- Do not call OpenAI, Gemini, or relay URLs from the browser.
- Do not store API keys in localStorage, IndexedDB, sessionStorage, URL params, or client-visible config.
- Do not use `setInterval`, repeated `setTimeout`, or looped fetch calls to observe generation task status.
- Do not render or log image base64 payloads.

## API integration

- Put request logic in a shared API client layer, not directly inside page components.
- Use backend model capability responses to drive parameter controls.
- Handle loading, empty, error, disabled, permission, and duplicate-submit states.
- Use branded or clearly named TypeScript ID types where practical for tenant, project, asset, task, provider, and model IDs.

## Verification

For frontend-affecting work, run:

- `npm run lint`
- `npm run type-check`
- `npm run test`
- `npm run build`

After P1 moves the frontend into `frontend/`, run these commands from `frontend/`.
