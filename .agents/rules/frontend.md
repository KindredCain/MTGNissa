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

## State ownership

- Put shareable navigation, submitted search, filters, sorting, and entity identifiers in the URL.
- Put remote entities and list pages in TanStack Query.
- Put transient dropdown, modal, card-face, hover, and focus state in the owning React component.
- Put only the unsaved deck content, section changes, quantities, dirty state, and leave-confirmation state in the deck editor store.
- Keep business data and request behavior independent from viewport size and presentation layout.

## Mobile compatibility constraints

- The initial release continues to follow the approved desktop behavior: five card columns at `1440 px`, four at `1366 px` and `1280 px`, and whole-page horizontal scrolling below `1280 px`. Do not invent or ship an unapproved phone layout.
- Structure every new component so a future tablet or phone layout can reuse the same data hooks, domain state, mutations, and validation without rewriting the business layer.
- Do not use the User-Agent to select layouts. Use CSS media or container queries for layout and input capability queries or event APIs for interaction differences.
- Prefer Pointer Events for pointer interaction. Do not bind core behavior only to mouse events.
- Any information or action exposed on hover must also be reachable by click, keyboard focus, or a detail view. Hover-only behavior is not allowed.
- Configure touch activation constraints for drag interactions so dragging does not conflict with scrolling or native long-press behavior.
- Every drag-and-drop business action must have a non-drag alternative such as an action button or move menu.
- Do not use whole-page `transform: scale()` or absolute positioning copied from a single design canvas to implement responsive behavior.
- Preserve the card aspect ratio and a stable loading placeholder. Lazy-load images, and do not create image requests for list items outside the active loading or rendering window.
- Keep modal, drawer, and popover business content independent from fixed desktop coordinates so the same content can later render as a phone full-screen view or bottom sheet.
- Use dynamic viewport units for viewport-bound overlays and reserve `env(safe-area-inset-*)` space where appropriate.
- Do not adapt wide deck sections, tables, or charts to phones by shrinking text, cards, or interaction targets. A future phone layout must use section switching, vertical cards, or scoped horizontal scrolling.
- Lazy-load charts and complex editing features. Cancel obsolete requests or ignore stale responses so an earlier response cannot overwrite newer filter or search state.

## API, build, and testing

- Generate frontend API types from the approved OpenAPI 3.1 contract. Do not maintain duplicate handwritten request and response DTOs when generated types exist.
- Treat frontend validation as user feedback only. The Go backend remains authoritative for business validation.
- In development, proxy `/api` and `/health` from Vite to the Go service. In production, build static assets for embedding into the Go binary; do not require a Node.js production process.
- Ensure browser-history deep links can be refreshed by keeping `/api` and `/health` outside the SPA fallback and returning the frontend entry point for unmatched frontend routes.
- Use Vitest and React Testing Library for component behavior, MSW for API scenarios, and Playwright for end-to-end browser coverage.
- Keep Playwright desktop coverage for `1280 x 720`, `1366 x 768`, and `1440 x 900`.
- Keep mobile-device Playwright projects from the first frontend milestone to detect script crashes, uncloseable overlays, hover-only controls, and touch-event conflicts. Passing these guard tests does not mean the phone visual design is approved.
