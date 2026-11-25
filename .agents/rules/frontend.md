# Frontend development rules

## Required technology

- Write frontend application code in TypeScript with strict type checking enabled.
- Use React as a client-rendered single-page application and Vite as the build tool.
- Use React Router Data Mode for browser-history routing, nested layouts, route error boundaries, and route-level lazy loading.
- Use TanStack Query for server state, including requests, cache invalidation, mutations, and cursor-based infinite loading. Do not copy server state into a general-purpose client store.
- Keep ordinary UI state in React components. Use Zustand only for complex unsaved deck-editor state that must be shared across editor components.
- Use CSS Modules and CSS Custom Properties for project styling. Build layouts with Grid, Flexbox, media queries, and container queries.
- Wrap Radix UI Primitives in project-owned UI components when accessible dialogs, popovers, tooltips, checkboxes, or similar primitives are needed. Do not introduce a complete visual component system that overrides the project prototypes.
- Use dnd-kit for deck drag-and-drop behavior and Apache ECharts for deck statistics. Lazy-load chart and complex editor code by route or feature.
- Use pnpm through Corepack. Run TypeScript type checking separately because Vite transpilation does not perform type checking.

## Code organization and style

- Use PascalCase for components and types, and `useXxx` for Hooks. For ordinary functions, variables, and filenames, follow the owning module's existing conventions and keep comparable members consistent.
- Use `snake_case` for API fields, parameters, and API action members defined by the contract; do not apply this requirement to all frontend code.
- Use the existing Prettier and ESLint setup for formatting and linting. Explain the reasons, assumptions, or limits behind non-obvious logic rather than narrating the code.
- Follow existing module patterns. Give shared abstractions a clear responsibility and actual reuse need; do not add layers speculatively.
- Reuse existing capabilities before adding dependencies. Add a third-party dependency only for a clear benefit, and avoid frameworks or tools with overlapping responsibilities.
- Centralize display-name mappings in `src/shared/display-names.ts`. Keep these mappings limited to names; option ranges, defaults, ordering, and request behavior belong in their business modules.

## State ownership

- Put shareable navigation, submitted search, filters, sorting, and entity identifiers in the URL.
- Put remote entities and list pages in TanStack Query.
- Put transient dropdown, modal, card-face, hover, and focus state in the owning React component.
- Put only the unsaved deck content, section changes, quantities, dirty state, and leave-confirmation state in the deck editor store.
- Keep business data and request behavior independent from viewport size and presentation layout.
- Keep reusable domain state, calculations, and validation independent from UI components, dialogs, and Toasts. Return results or domain errors for pages or interaction Hooks to present.
- Define cache and editor-session lifetimes explicitly. Background fetches must not overwrite unsaved edits; clear session-specific caches when the session ends.
- Protect asynchronous results according to their impact. Prevent loss of unsaved edits, incorrect writes, results applied to the wrong query or entity, and effects on sessions that have already ended. Use cancellation, version checks, or result merging where needed.
- Brief display inconsistencies that can be corrected by a subsequent fetch or refresh are acceptable when they do not affect persisted data or normal operation. Do not require request serialization or extra cache coordination solely to eliminate them.

## Mobile compatibility constraints

- Card-grid pages (ordinary and Token Set cards, global search, collection, wishlist) adapt column counts to available width without shrinking cards or enforcing a desktop minimum width. Other pages retain a `1280 px` minimum and whole-page horizontal scrolling below it. Do not introduce unrelated mobile layouts.
- Mobile compatibility constraints remain in force to preserve future adaptation; they do not require mobile pages or device-specific tests in the initial release. Keep shared data hooks, domain state, mutations, and validation separate from presentation.
- Do not use the User-Agent to select layouts. Use CSS media or container queries for layout and input capability queries or event APIs for interaction differences.
- Prefer Pointer Events for pointer interaction. Do not bind core behavior only to mouse events.
- Any information or action exposed on hover must also be reachable by click, keyboard focus, or a detail view. Hover-only behavior is not allowed.
- The mobile product will not offer deck editing. Do not require a mobile editor layout, touch dragging, or quantity editing. In the future mobile release, preserve the five create/edit/copy entry buttons visually but disable their actions, and redirect direct editor-route access to deck detail; see web/README.md.
- The initial release uses drag-and-drop for deck section moves. Non-drag alternatives are optional future enhancements, not a development requirement.
- Do not use whole-page `transform: scale()` or absolute positioning copied from a single design canvas to implement responsive behavior.
- Preserve the card aspect ratio and a stable loading placeholder. Lazy-load images, and do not create image requests for list items outside the active loading or rendering window.
- Keep modal, drawer, and popover business content independent from fixed desktop coordinates so the same content can later render as a phone full-screen view or bottom sheet.
- Use dynamic viewport units for viewport-bound overlays and reserve `env(safe-area-inset-*)` space where appropriate.
- Do not adapt wide deck sections, tables, or charts to phones by shrinking text, cards, or interaction targets. A future phone layout must use section switching, vertical cards, or scoped horizontal scrolling.
- Lazy-load charts and complex editing features. Cancel obsolete requests or ignore stale responses so an earlier response cannot overwrite newer filter or search state.

## API, build, and testing

- Generate frontend API types from the approved OpenAPI 3.1 contract and reuse generated DTO and enum types. Do not maintain duplicate handwritten API types or manually edit `src/api/schema.d.ts`. When an interface changes, synchronize its contract and callers, then run `corepack pnpm generate:api` and `corepack pnpm check:contract` from `web`.
- Treat frontend validation as user feedback only. The Go backend remains authoritative for business validation.
- In development, proxy `/api` and `/health` from Vite to the Go service. In production, build static assets for embedding into the Go binary; do not require a Node.js production process.
- Keep frontend source in `web` and Go resource hosting in `internal/ui`. Use `node scripts/build.mjs` from the repository root for a complete build: check and build the frontend, replace the generated `internal/ui/dist`, and compile with `-tags embed_ui`. Default Go builds remain backend-only for Vite development and tests.
- Ensure browser-history deep links can be refreshed by keeping `/api` and `/health` outside the SPA fallback and returning the frontend entry point for unmatched frontend routes.
- Use Vitest and React Testing Library for component behavior, MSW for API scenarios, and Playwright for end-to-end browser coverage.
- Keep Playwright desktop coverage for `1280 x 720`, `1366 x 768`, and `1440 x 900`.
- Mobile layouts, device-specific behavior, and mobile-device Playwright projects are outside the initial release. Future mobile editor restrictions are documented in web/README.md.
