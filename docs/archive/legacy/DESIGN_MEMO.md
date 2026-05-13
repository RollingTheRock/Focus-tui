# MEMO: Design Patterns for High-Density Professional Information Surfaces

**Date:** 2026-04-17  
**Context:** Deriving concrete design rules for a terminal overview pane that must support fast scanning, task resumption, and high information density without sacrificing readability.

---

## 1. Executive Summary

Professional tools that users inhabit for hours at a time (IDEs, trading terminals, ops dashboards, issue trackers) share a common design philosophy: **density is not the enemy; unmanaged density is**. The research below extracts repeatable patterns from VS Code, JetBrains, Bloomberg Terminal, Grafana/Datadog, Linear, and modern TUIs. The through-line is that expert users prefer high spatial density *if* the interface provides strong structural scaffolding: predictable spatial grouping, clear typographic hierarchy, progressive disclosure, and non-color semantic cues.

For a terminal overview pane, the most relevant analog is not a sparse marketing dashboard—it is the **IDE sidebar**, the **trading blotter**, and the **ops triage view**: surfaces where the user must re-establish context within 3–5 seconds of glancing at the screen.

---

## 2. Domain Case Studies & Patterns

### 2.1 IDEs: VS Code & JetBrains — The Sidebar as Context Engine

**Key insight:** The file explorer / tool window is not navigation; it is *spatial memory*. Users rely on position, indentation, and icon shape to resume work without reading.

| Pattern | Source / Example | Practical takeaway |
|---------|------------------|-------------------|
| **Tree indentation & guide lines** | JetBrains uses deeper, configurable indentation (8–16px+) with persistent vertical guide lines; VS Code has smaller defaults and users frequently file issues requesting JetBrains-style guides (see VS Code issue #305170). | Deeper indentation + guide lines reduce cognitive load in deep hierarchies. For a terminal tree/list, generous horizontal offset and subtle vertical connectors improve scanability. |
| **Icon + label alignment grids** | VS Code Explorer aligns file icons on a strict vertical grid; JetBrains uses type-specific icons (folders, files, test files) with consistent baselines. | Strict vertical alignment lets the eye "fall" down a column. Misalignment forces re-reading. |
| **Collapsible sections with state persistence** | VS Code restores sidebar visibility, expanded/collapsed nodes, and scroll position per workspace (docs: VS Code User Interface). | Resumption speed depends on layout persistence. The overview pane must remember open/closed groups and scroll position across sessions. |
| **Status embedding** | JetBrains overlays version-control status (color dots, modified badges) directly on file icons; VS Code does the same in the Explorer and Problems panels. | Embed micro-status *inside* the primary list item rather than in a separate column when space is tight. |
| **Tool window shortcuts over chrome** | JetBrains "New UI" removes numbers from tool-window icons and teaches keyboard shortcuts (Alt+1, Alt+4, etc.) to reclaim pixels (Helen Scott, 2024). | In a terminal overview, keyboard shortcuts for switching sections/panels maximize data area and reward muscle memory. |

**Sources:**
- VS Code UI Guidelines: https://code.visualstudio.com/docs/getstarted/userinterface
- VS Code Extension UX Guidelines (Views/Tree Views): https://code.visualstudio.com/api/ux-guidelines
- VS Code Issue #305170 (tree indentation): https://github.com/microsoft/vscode/issues/305170
- JetBrains UI Tips (Helen Scott): https://www.helenjoscott.com/2024/11/28/5-tips-for-your-jetbrains-ide-interface/

---

### 2.2 Terminal / TUI Dashboards — Constraints as Clarity

**Key insight:** Terminals have no affordances of the web (hover shadows, gradients, rounded corners). Clarity must come from character grids, box-drawing borders, color semantics, and fixed spatial zones.

| Pattern | Source / Example | Practical takeaway |
|---------|------------------|-------------------|
| **Fixed spatial zones (status → results → hints)** | `jfr-shell --tui` uses a rigid top-to-bottom layout: status bar, results pane (table/tree), command input, hints bar (JFR Shell blog, 2026). | A terminal overview should reserve a bottom "hint/status" line and a top "context" line so the data pane is visually stable. |
| **Braille spinners & sparklines inline** | Ratatui/Bubble Tea dashboards use `⠋⠙⠹⠸` for loading and `▂▃▄▅▆▇█` for inline micro-charts (SCKelemen/tui; Smithers TUI). | Inline sparklines communicate trend without expanding rows. Loading states should be small, peripheral, and animated with low-frequency updates to avoid distraction. |
| **Panel borders as grouping** | TUI frameworks (Rich, Textual, Ratatui) rely on `┌─┐│└┘` or block characters to create discrete regions. Asymmetric, weighted layouts are encouraged over single-column scrolling (terminal-ui-design skill). | Use borders sparingly but consistently to carve the screen into "cards." Whitespace inside panels is still necessary; a wall of un-bordered text is unreadable at terminal density. |
| **Keyboard-navigable dashboards** | Skrills TUI dashboard uses `R/A/T/M/S` keys to jump tabs; Recon (tmux dashboard) uses `1-4` to zoom into room grids (Skrills issue #147; Gentic.news Recon review). | Direct-key navigation beats arrow-key traversal for power users. Map sections to single keystrokes. |
| **Color-coded context bars** | Recon shows token usage as green/yellow/red horizontal bars inside each agent card (Gentic.news). | Horizontal bar segments (even 1-char tall blocks) make thresholds instantly readable without reading numbers. |

**Sources:**
- JFR Shell TUI write-up: https://jbachorik.github.io/posts/jfr-shell-tui
- SCKelemen/tui framework: https://github.com/SCKelemen/tui
- Skrills TUI Dashboard issue: https://github.com/athola/skrills/issues/147
- Recon (tmux dashboard): https://gentic.news/article/recon-the-tmux-dashboard-that-finally-makes-multi-agent-claude-code-workflows-manageable
- Terminal UI Design skill: https://agentskills.in/marketplace/@steveclarke/terminal-ui-design

---

### 2.3 Ops Dashboards: Grafana & Datadog — Triage at a Glance

**Key insight:** Incident-response dashboards are judged by **Mean Time To Recognition (MTTR)**. The best ones use a strict visual hierarchy and a "one page = one decision" rule.

| Pattern | Source / Example | Practical takeaway |
|---------|------------------|-------------------|
| **Z-pattern layout** | Grafana best-practice docs recommend placing critical status top-left, trends top-right, detailed breakdowns bottom-left, logs/alerts bottom-right (MetricFire; Grafana docs). | In a terminal overview, the top-left area is the highest-value real estate; place the most urgent summary there. |
| **5-color sequential palette max** | Grafana recommends limiting palettes and using semantic color (blue = good, red = bad, orange = warning) with high contrast (Grafana docs). | Terminals have 256 colors but should still limit to a tight semantic set: 1 neutral, 1 accent, 1 success, 1 warning, 1 error. |
| **Progressive disclosure: SLOs → service → endpoint → pod** | OpenObserve advocates leading with high-level SLO charts and drilling down to per-pod details (OpenObserve). | The overview pane should show a "rolled-up" state by default; individual items expand on demand (e.g., `Enter` to expand a session, `Esc` to collapse). |
| **Annotations / markers on timelines** | Grafana users mark deployments and incidents directly on charts; this provides causal context without leaving the dashboard. | In a terminal overview, inline "event dots" or brief timestamped tags next to trends show *when* something changed without needing a full log view. |
| **Consistent axes & units across panels** | Datadog and Grafana both stress that muscle memory depends on predictable scales and color order (OpenObserve; Datadog blog). | If the overview shows multiple metrics, align their scales or use consistent units; do not mix "seconds" and "milliseconds" in adjacent panels without conversion. |

**Sources:**
- Grafana Dashboard Best Practices: https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/best-practices
- MetricFire Grafana Guide: https://www.metricfire.com/blog/7-best-practices-for-grafana-dashboard-design
- OpenObserve Dashboard Patterns: https://openobserve.ai/resources/observability-dashboards
- Datadog Executive Dashboards: https://www.datadoghq.com/blog/datadog-executive-dashboards
- Dennis Henry Medium (incident response dashboards): https://medium.com/@dennishenry/designing-engineering-dashboards-for-incident-response-the-good-the-bad-and-the-ugly-f784bb17c4ee

---

### 2.4 Trading Terminals: Bloomberg & Modern Platforms — Attention as a Finite Resource

**Key insight:** Trading UIs are the extreme case of density + speed. Bloomberg’s design is famously "ugly" to outsiders but optimized for **pattern recognition at a glance**.

| Pattern | Source / Example | Practical takeaway |
|---------|------------------|-------------------|
| **Pattern-over-pixels** | Traders report they "don't focus on the screens, but look for patterns within the movement and colours" (Caplin Systems, 2010). | In a dense terminal overview, users will eventually read *shapes and colors* rather than text. Design for pre-attentive attributes: consistent status colors, aligned numbers, and repeatable row shapes. |
| **Keyboard-first, command-driven navigation** | Bloomberg started as a keyboard (`AAPL EQUITY GO`) and retains command-line entry over mouse navigation (SimpleFunctions, 2026). | A terminal overview should support command-like filters or quick jumps (e.g., `/search`, `1`–`9` section jumps) rather than forcing cursor navigation. |
| **High density, minimal padding** | Viraj Patel (2025) notes: "Whitespace might look great in Figma—to a trader, that’s wasted space." Compact grid components with readable typography are preferred. | Terminal TUIs naturally have tight line heights; use them. But maintain *micro* whitespace (a blank line between logical groups) to prevent the "wall of text" effect. |
| **Selective access, not unrestricted visibility** | T Trading (2026) argues that "more instruments visible" does not equal more power; it equals noise. Good platforms sequence information rather than surfacing all alerts simultaneously. | The overview pane should not show every log line. It should show *summaries* and *exceptions*, with full logs behind a deliberate action. |
| **CVD-safe color themes** | Bloomberg researched color-vision deficiency extensively and found ~20,000 users affected; they created alternative high-contrast color sets that preserved semantic meaning (Bloomberg UX, 2021). | Do not rely on red/green alone. Use text labels, symbols (`↑↓→`, `●○◐`), or patterns alongside color. |

**Sources:**
- Bloomberg UX: Designing for Color Accessibility: https://www.bloomberg.com/ux/2021/10/14/designing-the-terminal-for-color-accessibility/
- Bloomberg UX: Concealing Complexity: https://www.bloomberg.com/company/stories/how-bloomberg-terminal-ux-designers-conceal-complexity/
- Caplin Systems (Bloomberg makeover analysis): https://caplin.com/insights/posts/2010/04/07/duncan-on-the-impossible-bloomberg-makeover
- Psychology-Driven Layouts (Viraj Patel): https://medium.com/design-bootcamp/psychology-driven-layouts-designing-for-how-traders-think-b11e7cac5c
- Attention Is a Resource (T Trading): https://medium.com/t-trading-trading-platform-perspectives/attention-is-a-resource-what-trading-platforms-do-with-it-f91ee84a16fe
- SimpleFunctions (CLI trading): https://simplefunctions.dev/blog/why-best-trading-terminal-is-command-line

---

### 2.5 Productivity Workbenches: Linear & Notion — Calm Density

**Key insight:** Linear’s 2024–2026 redesigns explicitly optimized for "a calmer interface for a product in motion"—reducing visual noise so that dense information remains scannable.

| Pattern | Source / Example | Practical takeaway |
|---------|------------------|-------------------|
| **Dimmer navigation chrome** | Linear refreshed its sidebar to be "a few notches dimmer" so the main content area stands out (Linear blog, March 2026). | In a terminal overview, use lower-intensity colors (e.g., dark gray or dimmed text) for section headers and borders, reserving full brightness for data and status. |
| **Consistent header bars across views** | Linear unified headers, navigation, and view controls so orientation is instant when switching contexts (Linear changelog). | If the overview has multiple sub-views (e.g., sessions, tasks, metrics), keep the header structure identical—only the data changes. |
| **Block-based atomic units** | Notion treats every piece of content as a "block," making drag/reorder/nest operations predictable (HowWorks). | Structure the overview as discrete, addressable rows or cards. Each unit should be movable, collapsible, and independently refreshable without disrupting the whole layout. |
| **Progressive disclosure via side-peek / modals** | Notion opens database detail pages in a side panel rather than navigating away; Linear uses split views. | The overview pane should not navigate *away* from context. Use inline expansion, bottom panels, or modal overlays for detail views. |
| **Exception-first scanning** | Linear’s issue lists prioritize "Inbox" and "Triage"—surfaces that surface only what requires action. | The overview should default to surfacing exceptions, changes, and items needing attention, with stable/successful items visually receding. |

**Sources:**
- Linear UI Refresh Changelog: https://www.linear.app/changelog/2026-03-12-ui-refresh
- Linear Blog: A Calmer Interface: https://linear.app/now/behind-the-latest-design-refresh
- Linear Blog: Redesign Part II: https://linear.app/blog/how-we-redesigned-the-linear-ui
- How Notion Was Built: https://howworks.ai/blog/how-notion-was-built

---

## 3. Cross-Cutting Principles (The Five Dimensions)

### 3.1 Density
- **Contextual density:** Overview contexts benefit from *lower* density (4–5 key chunks) for pattern recognition; analysis contexts can support higher density (7–9 chunks) because the user is in focused attention (Sanjay Dey, 2026).
- **Expert users want density:** Frequency of use correlates with desired density. If users check the overview many times per day, they will tolerate—and prefer—more data on screen (Ruixen, 2025).
- **Card/panel boundaries:** Even in a terminal, discrete bordered regions ("cards") let users process each unit as a chunk without reading every line (Timothy Graf, 2026).

### 3.2 Grouping
- **Miller’s Law (4±1 chunks):** Working memory is limited. Group related items under clear section headers; do not present more than 5–7 ungrouped items in a single viewport (Boundev, 2026).
- **Spatial grouping over chromatic grouping:** Physical proximity and consistent internal alignment are stronger grouping cues than color. Use blank lines or border rules between groups, not just background tints.
- **Role-based grouping:** In Datadog/Grafana, metrics are grouped by function (latency together, errors together). In a terminal overview, group by *concern* (e.g., "Active Sessions," "Recent Alerts," "Resource Summary") rather than by raw data type.

### 3.3 Hierarchy
- **Z-pattern / F-pattern:** Place the most critical summary in the top-left. Secondary details flow down and right. The bottom of the screen is for hints, logs, and low-frequency actions (IGC, 2026).
- **Three-level hierarchy:**  
  1. **Primary:** Status / exceptions (high contrast, bright color, top-left).  
  2. **Secondary:** Supporting metrics / lists (neutral color, mid-contrast).  
  3. **Tertiary:** Navigation chrome / metadata (dimmed, low contrast).
- **Typography as hierarchy:** In terminal UIs, this translates to bold for primary labels, normal for values, and dim/faint for metadata. Limit to two type treatments (e.g., bold + normal) and one font family.

### 3.4 Scanability
- **Pre-attentive attributes:** Users scan for color, shape, alignment, and motion before reading text. Leverage: left-aligned identifiers, right-aligned numbers, consistent status-dot placement, and inline sparklines (SCKelemen/tui).
- **Exception-first design:** Humans are wired to look for what is wrong. If 95% of items are "green/OK," those items should visually recede so that the abnormal items pop (Eli Turner, 2026).
- **Sticky context:** In long lists, keep headers or current-path context visible. In a terminal, this may mean a fixed top bar or a persistent breadcrumb line.

### 3.5 Actionability
- **One page = one decision:** If the overview cannot answer "what do I do next?" in 5 seconds, split it or restructure it (OpenObserve; Boundev).
- **Inline actions, deferred details:** Show the action trigger (e.g., `[Enter] resume`, `[k] kill`) next to the item; hide the full command/output behind the action.
- **Keyboard shortcuts as primary actions:** Mouse/touch is secondary in terminal contexts. Every common action should have a single-key shortcut published in the bottom hints bar.

---

## 4. Concrete Design Rules for a Terminal Overview Pane

Derived from the above research, the following rules can be applied directly to the overview pane design:

1. **Fixed three-zone layout:**  
   - **Top line (context):** Current workspace / filter / time range.  
   - **Middle (data):** Divided into 2–4 bordered panels/cards.  
   - **Bottom line (hints):** Context-aware keyboard shortcuts (e.g., `q quit  r refresh  Enter open  ? help`).

2. **Panel priority (Z-pattern):**  
   - Top-left panel = most urgent / exception-heavy data.  
   - Top-right panel = secondary metrics or trends.  
   - Bottom panels = logs, lists, or historical context.

3. **Row design for scanability:**  
   - Format: `[status_icon] [name] [sparkline/trend] [value] [action_key]`  
   - Keep all status icons in a single left column; all numeric values right-aligned in a single right column.

4. **Color discipline:**  
   - Use a 4-color semantic palette: neutral (gray), info/accent (blue/cyan), warning (yellow), error (red).  
   - Do not use green for "OK" without an additional shape/text cue (accessibility / CVD safety).

5. **Progressive disclosure by default:**  
   - Each row shows a one-line summary.  
   - `Enter` expands the row into a detail view (inline or split-pane).  
   - `Esc` always collapses back to summary.

6. **State persistence = resumption speed:**  
   - Remember which panels were expanded, the scroll position, and the active filter across restarts.  
   - Restore the user to exactly the view they left.

7. **Keyboard-first navigation:**  
   - Numbered tabs or sections (`1`–`4` to jump panels).  
   - `/` to filter the current list.  
   - `j/k` or `↑/↓` for row navigation.

8. **Exception-first visual policy:**  
   - Normal / healthy items use dimmed or neutral styling.  
   - Exceptions (errors, high latency, stale data) use high-contrast, bright styling so they are the first thing seen.

9. **Borders and spacing:**  
   - Use box-drawing characters or block borders to separate panels.  
   - Maintain at least one blank line between unrelated groups inside a panel.

10. **Limit real-time motion:**  
    - Sparklines and status bars may update live, but avoid full-screen refreshes or flashy animations.  
    - Updates should be perceptible but not pull the eye away from the user’s current focus.

---

## 5. Selected Sources

| Source | URL |
|--------|-----|
| VS Code User Interface Docs | https://code.visualstudio.com/docs/getstarted/userinterface |
| VS Code Extension UX Guidelines | https://code.visualstudio.com/api/ux-guidelines |
| VS Code Issue #305170 (Tree Indentation) | https://github.com/microsoft/vscode/issues/305170 |
| Helen Scott — JetBrains IDE Interface Tips | https://www.helenjoscott.com/2024/11/28/5-tips-for-your-jetbrains-ide-interface/ |
| Evil Martians — 5 Essential Design Patterns for Dev Tool UIs | https://evilmartians.com/chronicles/keep-it-together-5-essential-design-patterns-for-dev-tool-uis |
| JFR Shell TUI Blog | https://jbachorik.github.io/posts/jfr-shell-tui |
| SCKelemen/tui Framework | https://github.com/SCKelemen/tui |
| Skrills TUI Dashboard (GitHub Issue) | https://github.com/athola/skrills/issues/147 |
| Recon TUI Dashboard Review | https://gentic.news/article/recon-the-tmux-dashboard-that-finally-makes-multi-agent-claude-code-workflows-manageable |
| Grafana Dashboard Best Practices | https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/best-practices |
| MetricFire — 7 Best Practices for Grafana | https://www.metricfire.com/blog/7-best-practices-for-grafana-dashboard-design |
| OpenObserve — Observability Dashboards | https://openobserve.ai/resources/observability-dashboards |
| Datadog — Effective Executive Dashboards | https://www.datadoghq.com/blog/datadog-executive-dashboards |
| Dennis Henry — Designing Engineering Dashboards for Incident Response | https://medium.com/@dennishenry/designing-engineering-dashboards-for-incident-response-the-good-the-bad-and-the-ugly-f784bb17c4ee |
| Bloomberg UX — Color Accessibility | https://www.bloomberg.com/ux/2021/10/14/designing-the-terminal-for-color-accessibility/ |
| Bloomberg UX — Concealing Complexity | https://www.bloomberg.com/company/stories/how-bloomberg-terminal-ux-designers-conceal-complexity/ |
| Caplin Systems — Bloomberg Makeover Analysis | https://caplin.com/insights/posts/2010/04/07/duncan-on-the-impossible-bloomberg-makeover |
| Viraj Patel — Psychology-Driven Layouts for Trading | https://medium.com/design-bootcamp/psychology-driven-layouts-designing-for-how-traders-think-b11e7cac5c |
| T Trading — Attention Is a Resource | https://medium.com/t-trading-trading-platform-perspectives/attention-is-a-resource-what-trading-platforms-do-with-it-f91ee84a16fe |
| SimpleFunctions — Why the Best Trading Terminal Is a Command Line | https://simplefunctions.dev/blog/why-best-trading-terminal-is-command-line |
| Linear — A Calmer Interface | https://linear.app/now/behind-the-latest-design-refresh |
| Linear — UI Refresh Changelog | https://www.linear.app/changelog/2026-03-12-ui-refresh |
| HowWorks — How Notion Was Built | https://howworks.ai/blog/how-notion-was-built |
| Srinath — Understanding UI Density | https://www.ruixen.com/blog/ui-density |
| Boundev — Dashboard Design Best Practices | https://www.boundev.com/blog/dashboard-design-best-practices-guide |
| Eli Turner — B2B UX: Optimize Cognitive Load | https://www.influencers-time.com/designing-b2b-ux-optimizing-cognitive-load-for-clarity/ |
| Timothy Graf — Reducing Cognitive Load in UI Design | https://timgraf.com/ux/reducing-cognitive-load-in-ui-design-essential-principles-for-modern-interfaces/ |
| IGC — Dashboard Layout & Visual Hierarchy | https://www.intelligentgraphicandcode.com/design/dashboard-design/dashboard-layout |

---

**End of memo.**
