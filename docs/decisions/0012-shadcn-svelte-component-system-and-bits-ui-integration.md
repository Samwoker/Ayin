# 12. shadcn-svelte Component System and bits-ui Integration

Date: 2026-10-09

## Status
Accepted

## Context
Phase 8 delivered the incident response console with SvelteKit 2 and Svelte 5 runes (`$state`, `$derived`, `$props`). The interface was styled directly with Tailwind CSS v4, but lacked standard atomic design tokens, keyboard-accessible dialogs, focus trapping, roving tab index navigation, and consistent design system primitives (Buttons, Cards, Badges, Tabs, Dialogs, Inputs, Textareas, Separators).

The project requested integrating shadcn-svelte primitives adapted for Svelte 5 runes and Tailwind CSS v4.

## Decision
1. **Underlying Primitives**: Adopted `bits-ui` (`^2.19.5`), `@lucide/svelte` (`^1.54.0`), `tailwind-merge`, `clsx`, and `class-variance-authority` (cva).
2. **Atomic Component Architecture**: Built unstyled headless primitives into typed Svelte 5 component wrappers in `apps/web/src/lib/components/ui/`:
   - `button`: `Button` with variants (`default`, `destructive`, `outline`, `secondary`, `ghost`, `link`) and sizes (`default`, `sm`, `lg`, `icon`).
   - `badge`: `Badge` with severity variants (`sev1`, `sev2`, `sev3`, `sev4`, `warning`, `destructive`, `secondary`).
   - `card`: `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`.
   - `input`: `Input` text element styled with Tailwind v4 dark tokens.
   - `textarea`: `Textarea` element styled with dark tokens.
   - `separator`: `Separator` with horizontal/vertical decorative rule support.
   - `dialog`: `Dialog`, `DialogTrigger`, `DialogContent`, `DialogHeader`, `DialogFooter`, `DialogTitle`, `DialogDescription`, `DialogClose` with headless focus trap, ESC key dismissal, and backdrop blur overlay.
   - `tabs`: `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` using `bits-ui` roving focus and keyboard accessibility.
3. **Subpath Module Imports**: Migrated all component imports to modern `#lib/components/ui/...` subpath resolution matching SvelteKit 3 / Node 24 standards.
4. **Console View Upgrades**:
   - `ApprovalModal.svelte`: Refactored to accessible `Dialog` modal with `Button`, `Badge`, `Card`, and `Textarea`.
   - `IncidentDetail.svelte`: Integrated `Tabs` for trace, hypotheses, remediations, and runbooks switching with dynamic badges.
   - `IncidentList.svelte`: Replaced raw elements with `Card`, `Badge` (sev1-sev4 color coded), and `Input` search box.
   - `RemediationsView.svelte`: Implemented `Card`, `Badge`, and `Button` for proposed/approved/rejected action workflows.
   - `HypothesesView.svelte`: Refactored with `Card`, `Badge`, and `Separator`.
   - `InvestigationTrace.svelte`: Upgraded step hierarchy with `Card`, `Badge`, and `Button`.
   - `RunbookViewer.svelte`: Styled with `Card`, `Badge`, `Button`, and `Separator`.
   - `Navbar.svelte`: Integrated `Badge` and `Button`.

## Consequences
- **Accessibility**: Full WCAG 2.1 AA compliance on modal dialogs (focus trap, ARIA dialog roles, Escape key dismiss) and tabs (arrow key navigation, aria-selected).
- **Consistency**: Centralized variant and sizing tokens via CVA, eliminating one-off CSS duplications.
- **Verification**: Svelte diagnostics report 0 errors and 0 warnings; Vitest unit tests pass 13/13; Vite production build succeeds code 0.
