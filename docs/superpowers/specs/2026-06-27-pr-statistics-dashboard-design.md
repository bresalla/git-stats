# PR Statistics Dashboard Design

**Date:** 2026-06-27  
**Feature:** PR timing statistics and aggregated metrics on the Delivery Flow dashboard  
**Status:** Design approved

## Overview

Enhance the `/delivery-flow` page to show **aggregated PR statistics** alongside individual PR details. Instead of a raw table of PRs, users will see summary metrics (average, median, min/max), percentile distributions, and breakdowns by repository, team, and author—all in a minimalist, scholarly visual style.

## Problem Statement

The current Delivery Flow page shows individual PRs in a table with their cycle times, but does not provide aggregated insights. Engineering leads need to understand:
- How long PRs typically take to merge (on average, at various percentiles)
- How review timing compares across repositories and teams
- Whether there are outliers or patterns in PR velocity

## Goals

1. Surface aggregate PR metrics (average, median, percentiles) at a glance
2. Enable breakdown analysis by repository, team, and author
3. Maintain minimalist, scholarly aesthetic (clean typography, monochromatic, text-centric)
4. Keep individual PR table for detail drill-down
5. Preserve existing filter functionality (repo, author, date range, PR state)

## Non-Goals

- Time-series trends or forecasting
- Individual developer performance ranking
- Colored visualizations or complex charts
- Real-time alerts or anomaly detection

## Design: Page Layout

The `/delivery-flow` page uses a **linear, top-to-bottom flow**:

### 1. Filters (Existing)
Unchanged. Users filter by repository, author, date range, and PR state. All statistics below respond to these filters.

### 2. Summary Cards (New)

Three equal-width KPI cards at the top:

**Card 1: Avg Cycle Time**
```
Avg Cycle Time
24.5 hours
Median: 18h | Min: 2h | Max: 168h
```

**Card 2: Avg Time to First Review**
```
Avg Time to First Review
6.3 hours
Median: 4h | Min: 0.5h | Max: 72h
```

**Card 3: Avg Time in Review**
```
Avg Time in Review
18.2 hours
Median: 14h | Min: 1h | Max: 144h
```

Each card displays:
- **Large number:** The average value (primary metric)
- **Small text below:** Median, minimum, and maximum (secondary insights)

**Styling:** White background, minimal border, generous padding. No color coding or icons. Serif or sans-serif typography matching the reference aesthetic.

### 3. Distribution Visualizations (New)

Three simple tables showing percentile breakdowns for each metric.

**Distribution Table: Cycle Time**
| Percentile | Hours |
|---|---|
| Median (P50) | 18 |
| P75 | 32 |
| P90 | 56 |
| P95 | 72 |
| P99 | 168 |

**Distribution Table: Time to First Review**
| Percentile | Hours |
|---|---|
| Median (P50) | 4 |
| P75 | 8 |
| P90 | 24 |
| P95 | 48 |
| P99 | 72 |

**Distribution Table: Time in Review**
| Percentile | Hours |
|---|---|
| Median (P50) | 14 |
| P75 | 22 |
| P90 | 48 |
| P95 | 56 |
| P99 | 144 |

**Styling:** Plain tables with minimal borders. Text-only, no graphics. Light gray header, black text on white background.

### 4. Toggleable Breakdown Tables (New)

Three collapsible sections stacked vertically. Users click section headers to expand/collapse. Each section shows the same three metrics (Avg Cycle Time, Avg Time to First Review, Avg Time in Review) broken down by a dimension.

**Section 1: By Repository** (Expandable)
| Repository | Avg Cycle Time | Avg 1st Review | Avg in Review |
|---|---|---|---|
| repo-core | 18h | 4h | 14h |
| repo-api | 22h | 8h | 14h |
| repo-web | 28h | 6h | 22h |

Show top 10 repositories by PR count. Rows sorted by Avg Cycle Time (longest first).

**Section 2: By Team** (Expandable)
| Team | Avg Cycle Time | Avg 1st Review | Avg in Review |
|---|---|---|---|
| Backend | 20h | 5h | 15h |
| Frontend | 26h | 7h | 19h |
| DevOps | 16h | 3h | 13h |

Show all teams with PRs in the filter range. Rows sorted by Avg Cycle Time (longest first).

**Section 3: By Author** (Expandable)
| Author | Avg Cycle Time | Avg 1st Review | Avg in Review |
|---|---|---|---|
| Alice | 16h | 3h | 13h |
| Bob | 22h | 6h | 16h |
| Carol | 24h | 8h | 16h |

Show top 10 authors by PR count. Rows sorted by Avg Cycle Time (longest first).

**Styling:**
- Section header: Bold text, minimal indicator (e.g., "▼ By Repository" when expanded, "▶ By Repository" when collapsed)
- Click anywhere on header to toggle
- Tables use same styling as distribution tables
- Smooth collapse/expand animation (or instant toggle if simpler)

### 5. Individual PR Table (Existing)

Unchanged. Displays all merged PRs matching the filter, with columns:
- Pull Request (Title)
- Cycle Time (h)
- Time to First Review (h)
- Time in Review (h)

Only merged PRs are shown (existing behavior).

## Data & Calculations

All statistics are computed **server-side** (in the `metrics` package) based on merged PRs in the filter range.

**Metrics calculated:**
- **Cycle Time:** `MergedAt - CreatedAt`
- **Time to First Review:** `FirstReviewAt - CreatedAt` (0 if no reviews)
- **Time in Review:** `MergedAt - FirstReviewAt` (0 if no reviews)

**Aggregations:**
- **Average, Median:** Standard statistical functions
- **Min, Max:** Minimum and maximum values
- **Percentiles (P50, P75, P90, P95, P99):** Use standard percentile calculation (e.g., linear interpolation or nearest-rank)

**Filters apply to all:** Repository, author, date range, and PR state filters affect all summary cards, distributions, and breakdown tables.

## Visual Aesthetic

**Color & Typography:**
- **Background:** Off-white (#f9f9f9) or light gray (#f5f5f5)
- **Text:** Black (#000) or dark gray (#333) on light backgrounds
- **Borders:** Light gray (#ddd) or #ccc, minimal thickness (1px)
- **Typography:** Clean sans-serif (e.g., system fonts: -apple-system, BlinkMacSystemFont, "Segoe UI", or serif alternative) with clear hierarchy
  - Headings: Bold, larger size
  - Body/table text: Regular weight, readable size (14-16px)
  - Metric numbers in cards: Larger, bold

**Styling principles:**
- No color coding (red/yellow/green for performance)
- No icons or decorative elements
- Generous whitespace and padding
- Minimal borders and separation lines
- Scholarly, text-centric presentation

## Implementation Notes

### Backend Changes

Update `internal/metrics/metrics.go`:
- Add new metric calculation function for PR summary stats
  - Input: filter (repo, author, date range, PR state)
  - Output: struct with Average, Median, Min, Max for all three metrics
- Add percentile calculation function
  - Input: slice of durations (cycle times, review times, etc.)
  - Output: P50, P75, P90, P95, P99
- Add breakdown functions
  - By repository: group PRs by `RepoSlug`, calculate metrics per group
  - By team: group PRs by team (infer from author → team mapping), calculate metrics per group
  - By author: group PRs by `AuthorID`, calculate metrics per author

### Frontend Changes

Update `internal/web/handlers.go`:
- Modify `handleDeliveryFlow` to fetch new metrics (summary, distributions, breakdowns)
- Build page data struct with all sections

Update `internal/web/templates/delivery_flow.html`:
- Add summary card section
- Add distribution tables section
- Add toggleable breakdown sections (using minimal CSS for collapse/expand)
- Keep existing PR table at bottom
- Update styling to match minimalist aesthetic

### Database

No schema changes needed. All calculations derive from existing `pull_requests`, `reviews`, and `authors` tables.

## Scope & Constraints

**In Scope:**
- Summary cards (avg, median, min, max)
- Percentile distribution tables (P50, P75, P90, P95, P99)
- Breakdown tables (by repo, team, author—top 10 each where applicable)
- Minimalist UI styling
- Toggleable sections
- Existing filter integration

**Out of Scope:**
- Time-series trends
- Colored charts or complex visualizations
- Individual ranking or "performance" framing
- Alerts or anomalies
- Export functionality for the new stats (keep existing CSV export)
- Mobile optimization (nice-to-have, not required)

## Success Criteria

1. Summary cards display accurate average, median, min, max for all three metrics
2. Percentile distributions match statistical definitions
3. Breakdown tables show meaningful groupings (top repos, all teams, top authors)
4. UI matches minimalist, scholarly aesthetic (no colors, clean typography)
5. Toggleable sections collapse/expand without page reload
6. Filters (repo, author, date, state) correctly affect all new sections
7. Individual PR table remains functional and unchanged
8. All calculations are server-side; page loads in <1s for typical dataset

## Testing Strategy

- Unit tests for metric calculations (average, median, percentiles)
- Integration tests for breakdown grouping and aggregation
- Web handler tests to ensure correct data flows to template
- Manual testing of filter interactions (all combinations of filters)
- Visual regression testing to ensure styling matches reference

## Future Enhancements

- Sort/reorder breakdown tables by clicking column headers
- Drill-down: click a repository/team/author row to filter the PR table
- Export breakdown statistics as CSV
- Time-series microtrends (small sparklines, if data supports)
- Comparison mode (e.g., "team A vs team B" side-by-side)
