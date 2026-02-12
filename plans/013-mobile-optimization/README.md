# Plan 013: Mobile Optimization

## Quick Start

1. **Read the main plan**: `013-mobile-optimization.md` (1,667 lines)
2. **Review PRD tasks**: `prd.json` (673 lines, 62 actionable steps)
3. **Track progress**: `progress.txt` (update as you complete phases)

## Plan Overview

Transform Kuberan into a fully mobile-responsive application optimized for devices from 375px to 768px width.

### Scope
- **40-45 files** to modify (frontend-only, zero backend changes)
- **8 phases** of work (4 critical/high priority, 4 medium priority)
- **62 actionable steps** in the PRD
- **Estimated time**: 22-32 hours of development

### Target Devices
- iPhone SE (375px) — primary test device
- iPhone 12-14 (390px)
- iPhone 14 Pro Max (428px)
- iPad Mini (768px)

## Implementation Phases

### Phase 1: Foundation & Core Layout (CRITICAL)
**Time**: 2-3 hours  
**Files**: 6 files (globals.css, button.tsx, layout files)

- Fix touch target sizes (44px minimum)
- Add touch-action CSS optimization
- Verify sidebar drawer functionality
- Prevent horizontal scroll

**Start here**: This phase fixes critical usability issues that affect every page.

### Phase 2: Table to Card/List Conversions (HIGH)
**Time**: 4-6 hours  
**Files**: 3 files (accounts, transactions, categories pages)

- Convert desktop tables to mobile card layouts
- Add collapsible filter panels
- Icon-only pagination buttons

**Impact**: Makes all list pages usable on mobile.

### Phase 3: Dashboard & Chart Responsiveness (HIGH)
**Time**: 3-4 hours  
**Files**: 5 files (dashboard page + 4 chart components)

- Reduce chart heights on mobile
- Prevent label overlap
- Stack header action buttons

**Impact**: Dashboard becomes readable and functional on small screens.

### Phase 4: Dialogs & Forms (HIGH)
**Time**: 4-5 hours  
**Files**: 11+ files (all dialog components)

- Stack button groups on mobile
- Increase form field spacing
- Ensure adequate touch targets

**Impact**: All forms become easy to fill on mobile.

### Phase 5: Detail Pages (MEDIUM)
**Time**: 3-4 hours  
**Files**: 3 files (account, investment, security detail pages)

- Stack headers vertically
- Responsive stat card grids
- Mobile transaction lists

### Phase 6: Remaining Pages (MEDIUM)
**Time**: 2-3 hours  
**Files**: 3 files (budgets, investments, securities pages)

- Optimize filter layouts
- Convert remaining tables to cards
- Responsive chart heights

### Phase 7: Component Audit (MEDIUM)
**Time**: 2-3 hours  
**Files**: 6 files (UI primitive components)

- Verify input/select heights
- Responsive card padding
- Touch target compliance

### Phase 8: Final Verification & Testing (CRITICAL)
**Time**: 2-4 hours  
**Files**: N/A (testing phase)

- Visual verification checklist (60+ items)
- Accessibility checks
- Browser testing (Safari iOS, Chrome Android)
- Regression testing (desktop still works)

## Quick Reference

### Key Design Decisions
- **Mobile breakpoint**: 768px (md:)
- **Touch target minimum**: 44×44px
- **Form field spacing**: gap-5 (20px)
- **Card padding**: p-4 on mobile, p-6 on desktop
- **Chart heights**: 200px on mobile, 250px on desktop

### Common Patterns

#### Responsive Table/Card Switching
```tsx
{/* Mobile: Cards */}
<div className="md:hidden">
  <div className="grid gap-3">{cards}</div>
</div>

{/* Desktop: Table */}
<div className="hidden md:block">
  <Table>{/* ... */}</Table>
</div>
```

#### Collapsible Filter Panel
```tsx
const [showFilters, setShowFilters] = useState(false);
const activeFilterCount = /* count active filters */;

<Button onClick={() => setShowFilters(!showFilters)}>
  Filters {activeFilterCount > 0 && <Badge>{activeFilterCount}</Badge>}
</Button>
{showFilters && <FilterPanel />}
```

#### Button Group Stacking
```tsx
<div className="flex flex-col sm:flex-row gap-2">
  <Button className="flex-1">Option 1</Button>
  <Button className="flex-1">Option 2</Button>
</div>
```

## Verification Commands

After each phase:
```bash
cd apps/web
pnpm build
```

No backend changes, so no need to run:
```bash
./scripts/check-go.sh apps/api  # NOT NEEDED
```

## Testing Checklist

Use browser DevTools device emulation or real devices:

### Critical Tests (Required)
- [ ] No horizontal scroll at 375px width
- [ ] All buttons tappable (44px minimum)
- [ ] Tables become cards on mobile
- [ ] Filters accessible via collapsible panel
- [ ] Charts readable on small screens
- [ ] Forms have adequate spacing
- [ ] Sidebar works as mobile drawer

### Regression Tests (Required)
- [ ] Desktop experience still works (1024px+)
- [ ] Tables visible on desktop
- [ ] Button groups horizontal on desktop
- [ ] Filters horizontal on desktop

## Success Criteria

Mobile optimization is **COMPLETE** when:

1. ✅ Zero TypeScript build errors
2. ✅ All pages load without horizontal scroll (375px-768px)
3. ✅ All interactive elements meet 44×44px touch target minimum
4. ✅ Charts readable and interactive on mobile
5. ✅ Forms and dialogs have adequate spacing
6. ✅ Filters accessible via collapsible panels
7. ✅ No critical UX issues on 4 target device sizes
8. ✅ Desktop experience still fully functional

## Files Changed Summary

### New Files
**0 new files** — all modifications to existing files

### Modified Files by Category

**Foundation (6 files)**
- globals.css
- layout.tsx (verify)
- button.tsx
- app-header.tsx (verify)
- app-sidebar.tsx (verify)
- (dashboard)/layout.tsx (verify)

**Tables (3 files)**
- accounts/page.tsx
- transactions/page.tsx
- categories/page.tsx

**Charts (5 files)**
- (dashboard)/page.tsx
- expenditure-chart.tsx
- income-expenses-chart.tsx
- spending-trend-chart.tsx
- net-worth-chart.tsx

**Dialogs (11+ files)**
- create-transaction-dialog.tsx
- edit-transaction-dialog.tsx
- create-account-dialog.tsx
- edit-account-dialog.tsx
- create-budget-dialog.tsx
- edit-budget-dialog.tsx
- create-category-dialog.tsx
- edit-category-dialog.tsx
- 5× investment dialogs

**Detail Pages (3 files)**
- accounts/[id]/page.tsx
- investments/[id]/page.tsx
- securities/[id]/page.tsx

**Remaining Pages (3 files)**
- budgets/page.tsx
- investments/page.tsx
- securities/page.tsx

**Components (6 files)**
- input.tsx
- select.tsx
- textarea.tsx
- currency-input.tsx
- card.tsx
- badge.tsx (verify)

**Total**: ~40-45 files

## Next Steps

1. **Start with Phase 1** — Foundation changes affect all pages
2. **Test after each phase** — Run `pnpm build` and visual verification
3. **Update progress.txt** — Mark phases as complete
4. **Complete Phase 8** — Full verification before considering done

## Questions?

Refer to:
- Main plan document for detailed implementation instructions
- PRD JSON for step-by-step task breakdown
- CLAUDE.md in repo root for project architecture context
