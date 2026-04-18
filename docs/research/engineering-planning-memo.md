# How Top-Tier Programmers Plan Complex Software Work

## Research Memo for Focus-tui

*Date: 2026-04-18*
*Sources: Public engineering blogs, RFC repositories, technical talks, and published practices from Uber, Google, Stripe, Shopify, Rust, React, and respected individual engineers.*

---

## 1. Planning Hierarchy: From Intent to Implementation

### The Core Insight

Top engineering teams use a **tiered document hierarchy** that separates "what we decided and why" from "how we implement it." The hierarchy typically looks like:

```
PRD / Product Spec          → What problem are we solving?
    ↓
RFC / Design Doc            → What is the proposed technical approach?
    ↓
ADR                         → What specific architectural decision did we make, and why?
    ↓
Implementation Plan         → Phases, milestones, dependencies, owners
    ↓
Tasks / Issues              → Concrete, verifiable units of work
```

### Evidence from Practice

**Uber's evolution** is the canonical case study. They started with "DUCKs" (Designs Uber Creates Knowledge) for new services, evolved into RFCs as they crossed ~100 engineers, and later introduced a **tiered Engineering Planning Framework** with explicit levels of rigor as they grew past 2,000 engineers (Gergely Orosz, *The Pragmatic Engineer*).

> "For small changes: don't bother. Just make the change. For changes that are non-trivial and have dependencies: consider writing one. The effort to write an RFC should be proportionate to the complexity of the task."
> — Gergely Orosz, "Engineering Planning with RFCs, Design Documents and ADRs"
> https://newsletter.pragmaticengineer.com/p/rfcs-and-design-docs

**Google** requires an approved design document before any major project. Their canonical template forces engineers to consider security, internationalization, storage, privacy, and testing *before* code is written (*Software Engineering at Google*, Chapter 10).

**PointFive** uses two templates calibrated to scope:
- **Tech-Spec RFC**: Full spec for "Big Rock" projects — includes implementation plan, phased rollout, milestones, dependencies, testing strategy, API/database changes, architecture overview.
- **Mini RFC**: Lightweight format for smaller changes — covers problem, proposed change, open questions, implementation details, tradeoffs.
> https://www.pointfive.co/blog/writing-technical-specifications-the-art-of-tailoring-rfcs

**Rust's RFC process** uses GitHub PRs as a state machine. Proposals start as markdown files from a template, are submitted as PRs to the `rust-lang/rfcs` repo, and move through labeling, sub-team assignment, discussion, a mandatory 10-day "Final Comment Period," and finally merge/reject. The template includes: Summary, Motivation, Guide-level explanation, Reference-level explanation, Drawbacks, Rationale and alternatives, Prior art, Unresolved questions, Future possibilities.
> https://github.com/rust-lang/rfcs/blob/master/0000-template.md

**React's RFC process** (inspired by Rust, Yarn, and Ember) similarly uses a template with: Summary, Basic example, Motivation, Detailed design, Drawbacks, Alternatives, Adoption strategy, How we teach this, Unresolved questions.
> https://github.com/reactjs/rfcs/blob/main/0000-template.md

---

## 2. Decomposition: Breaking Down Complex Work

### The Core Insight

**John Ousterhout** (Stanford, author of *A Philosophy of Software Design*) argues that the single most important idea in computer science is **decomposition** — "how do you take large complicated problems and break them up." Software design is fundamentally a decomposition problem: you break a system into modules that can be implemented relatively independently.

> "Software design is a decomposition problem. How do you take a large system and divide it into smaller units that you can implement relatively independently? ... To me I think decomposition — that's the key thing that threads through everything we do in computer science."
> — John Ousterhout, *The Pragmatic Engineer* podcast, 2025
> https://newsletter.pragmaticengineer.com/p/the-philosophy-of-software-design

### Evidence from Practice

**GitHub's sub-issues** (launched 2025) were built because "as projects grow in complexity, breaking down work into smaller, actionable steps becomes essential." They support hierarchical lists within issues, making it easier to track progress and dependencies. The hierarchical structure "made it easier to identify dependencies and ensure nothing fell through the cracks."
> https://github.blog/engineering/architecture-optimization/introducing-sub-issues-enhancing-issue-management-on-github/

**The CCPM (Claude Code Project Manager)** system uses a strict 5-phase decomposition:
1. **Product Planning** → PRD
2. **Implementation Planning** → Epic with architectural decisions, technical approach, dependency mapping
3. **Task Decomposition** → Concrete tasks with acceptance criteria and effort estimates; flags parallelizable work
4. **GitHub Synchronization** → Push to issues with labels and relationships
5. **Execution** → Specialized agents implement individual tasks
> https://cc.deeptoai.com/docs/en/tools/ccpm-claude-code-project-manager

**GitHub's `awesome-copilot` breakdown-plan skill** defines an explicit hierarchy:
- **Epic**: Large business capability spanning multiple features
- **Feature**: Deliverable user-facing functionality
- **Story/Enabler**: User-facing work or technical infrastructure
- **Test**: QA work for validation
- **Task**: Implementation-level breakdown
> https://github.com/github/awesome-copilot/blob/main/skills/breakdown-plan/SKILL.md

**Shopify's React Native New Architecture migration** decomposed a massive codebase migration into phases: audit dependencies, upgrade RN version, enable Fabric in dev, minimize code changes, migrate modules strategically. Each phase had concrete entry/exit criteria.
> https://shopify.engineering/react-native-new-architecture

---

## 3. Dependency Handling

### The Core Insight

Dependencies are where plans most often fail. Top teams make dependencies **explicit, visible, and serialized** early in the planning process. They use the plan itself to identify what must happen in sequence vs. what can run in parallel.

### Evidence from Practice

**Uber's DUCKs** (the predecessor to RFCs) were explicitly designed to "help teams discover dependencies — often before starting to code." As the org grew, this dependency discovery became a primary motivation for formalizing the planning process.
> https://blog.pragmaticengineer.com/scaling-engineering-teams-via-writing-things-down-rfcs/

**The Tuson Advisory RFC playbook** includes explicit sections for:
- "Impacted systems and owners"
- "Rollout plan and migration strategy"
> https://tusonadvisory.com/when-you-cant-fit-engineering-in-one-room-anymore-a-practical-rfc-playbook/

**CCPM's decomposition phase** explicitly "identifies and flags tasks that can be worked on in parallel." The coordinator pattern in enterprise agentic workflows analyzes codebase context before breaking specs into "parallelizable task waves, ensuring shared dependencies are serialized while independent work streams run concurrently."
> https://www.augmentcode.com/guides/how-do-enterprise-teams-build-agentic-workflows

**AWS's ADR guidance** notes that ADRs should capture dependencies (coupling of components) as architecturally significant decisions that affect the software project.
> https://docs.aws.amazon.com/prescriptive-guidance/latest/architectural-decision-records/adr-process.html

**GitHub issue dependencies** (native feature) let teams "easily see and communicate which issues are blocked by, or blocking, other issues. This helps streamline coordination, prevent bottlenecks and increase transparency."
> https://help.github.com/en/issues/tracking-your-work-with-issues/planning-and-tracking-work-for-your-team-or-project

---

## 4. Risk Management

### The Core Insight

Risk management in engineering planning is not about eliminating risk — it's about **making risks visible, quantifying them, and assigning ownership** before implementation begins. The best teams treat planning as a bet on probability, not a guarantee of outcomes.

### Evidence from Practice

**Shopify's "Planning in Bets"** framework (for BFCM preparation) breaks risk mitigation into four questions:
1. **What are the risks?** — Run "what could go wrong" (WCGW) exercises across the platform.
2. **What is worth mitigating?** — Vote on risks, then have technical experts discuss likelihood and severity of the highest-ranked ones.
3. **Who makes what decisions?** — Identify decision makers, empower them to gather input and decide. "Often the decision is best made by the subject matter expert or who bears the consequences."
4. **How do you communicate?** — Summarize findings, share with stakeholders, maintain alignment.

> "The best way to deal with uncertainty is using probability. Expert poker players know that great bets don't always yield great outcomes and bad bets don't always yield bad outcomes. What's important is to bet on the probability of outcomes, where over enough rounds, your results will converge to expectation."
> — Kathryn Tang, Shopify Engineering Operations
> https://shopify.engineering/risk-mitigation-at-scale

**Shopify's React Native migration** defined an explicit Emergency Response Plan with three tiers based on stability thresholds:
1. Stability above 99.80% → fix forward on next weekly release
2. Stability between 99.00% and 99.80% → pause rollout and hotfix
3. Stability below 99.00% → rollback

Rollback was treated as a last resort because it had "severe consequences for our migration timeline."
> https://shopify.engineering/react-native-new-architecture

**SPH's RFC guide** emphasizes that implementation phases are "not just project management — it's risk management. Phase 1 delivers basic functionality, Phase 2 adds advanced features, and Phase 3 evaluates."
> https://sph.sh/en/posts/writing-effective-rfcs-principal-engineer-guide

**Stripe's "Minions"** (autonomous AI agents) address risk through architecture: sandbox isolation, zero-trust permissions, a two-round CI cap (circuit breaker), and deterministic control gates. "The winning AI implementation is not the smartest model but the best-governed infrastructure."
> https://medium.com/@oracle_43885/how-stripe-built-secure-unattended-ai-agents-merging-1-000-pull-requests-weekly-1ff42f3fe550

**AWS's ADR best practices** (from 200+ ADRs across projects):
- Keep ADR meetings short and focused
- Focus on a single decision per ADR
- Record the confidence level of the decision
- Separate design from decision (use a design doc to explore options)
> https://aws.amazon.com/blogs/architecture/master-architecture-decision-records-adrs-best-practices-for-effective-decision-making/

---

## 5. Validation Planning

### The Core Insight

Validation is not an afterthought — it is **designed into the plan from the beginning**. Top teams specify "how will we know this works?" before writing code. This includes testing strategy, monitoring, observability, and explicit success criteria.

### Evidence from Practice

**Google's design doc template** requires engineers to consider testing strategy, security implications, privacy concerns, and storage requirements as part of the initial design. "The best design documents suggest design goals and cover alternative designs, denoting their strong and weak points." The design doc, once approved, "acts not only as a historical record, but as a measure of whether the project successfully achieved its goals."
> https://abseil.io/resources/swe-book/html/ch10.html

**PointFive's Tech-Spec RFC** includes a dedicated "Testing strategy — How we will validate correctness and performance" section.
> https://www.pointfive.co/blog/writing-technical-specifications-the-art-of-tailoring-rfcs

**Google Cloud's change management model** has four phases: design, development, qualification, and rollout. The qualification phase is mandatory — "a multi-layered development and testing approach with manual and automated validation."
> https://docs.cloud.google.com/docs/cloud-approach-to-change

**Shopify's validation gates** for the React Native migration included:
- Shadow verifier to confirm migrated fields match legacy counterparts
- Benchmark suite for studying performance of migrated queries
- Stability targets (99.95% crash-free sessions)
- Rollout percentages with explicit go/no-go criteria per day
> https://shopify.engineering/react-native-new-architecture

**Stripe's CI system** runs "tens of thousands of test suites" with selective test execution for a 50M-line monorepo. Validation is automated and continuous.
> https://stripe.dev/blog/selective-test-execution-at-stripe-fast-ci-for-a-50m-line-ruby-monorepo

**The CCPM framework** includes acceptance criteria at the task level and automated test generation. "75% reduction in bug rates due to detailed task breakdown."
> https://cc.deeptoai.com/docs/en/tools/ccpm-claude-code-project-manager

---

## 6. How Plans Evolve During Execution

### The Core Insight

**No plan survives contact with the enemy.** Top teams treat plans as **living documents** that are updated during implementation, not write-once artifacts. They build in explicit triggers for replanning and preserve context when plans change.

### Evidence from Practice

**SPH's RFC guide**: "The RFC isn't done when it's approved — it's a living document that should evolve with implementation realities."
> https://sph.sh/en/posts/writing-effective-rfcs-principal-engineer-guide

**PointFive**: "Treat RFCs as living documents — Plans change as you learn. We update RFCs during implementation to reflect reality."
> https://www.pointfive.co/blog/writing-technical-specifications-the-art-of-tailoring-rfcs

**The PrimeLine "Universal Planning Framework"** separates discovery from planning:
- **Stage 0 (Discovery & Sparring)**: 10 checks before writing a single line of plan. "The 30 minutes you spend on discovery saves hours of execution on the wrong thing."
- **Stage 1 (The Plan)**: 5 core + 14 conditional sections. Phases break work into 3-4 hour chunks with binary gates.
- **Stage 2 (Meta Review)**: Anti-patterns, delegation, research gates.

Result: "Before the framework: roughly 40% of my plans needed significant replanning mid-execution. After: that dropped to under 10%. The plans that do need changes hit a built-in replanning trigger instead of a crisis."
> https://primeline.cc/blog/planning-with-claude-code

**TurinTech's Artemis** framework treats replanning as a collaboration feature:
- Make plans living documents, not write-once artifacts
- Preserve context during replanning (reset confidence scores, but preserve codebase exploration)
- Iterate until aligned: Generate plan → review → request adjustments → get revised version
- "For production systems where humans are accountable for outcomes, control often matters more than autonomy."
> https://turintech.ai/blog/the-art-and-science-of-a-good-plan-turning-requirements-into-reliable-results

**CrewAI's adaptive replanning** feature shows how even agentic systems need replanning triggers:
- After each task completes, evaluate: "Does this result deviate significantly from what the plan assumed?"
- If yes, generate a revised plan for remaining tasks only
- Cap replanning loops with `max_replans`
> https://github.com/crewAIInc/crewAI/issues/4983

**Plandek's research** on the planning-delivery gap:
> "Jira primarily captures intent. It shows what should happen... Execution, however, is far messier... The planning–delivery gap: the difference between a roadmap that looks healthy and a delivery system that actually is."
> https://plandek.com/blog/jira-planning-vs-execution-how-engineering-leaders-predict-what-will-actually-ship/

---

## 7. The RFC ↔ ADR Distinction

A critical pattern across all researched teams is the separation between **exploration documents** (RFCs / Design Docs) and **decision records** (ADRs):

| Aspect | RFC / Design Doc | ADR |
|--------|-----------------|-----|
| **Purpose** | Explore options, invite feedback, propose approach | Record what was decided and why |
| **Audience** | Team, stakeholders, reviewers | Future engineers, maintainers, auditors |
| **When** | Before decision | At or immediately after decision |
| **Length** | Can be long (2-20 pages) | Must be short (1 page, 10-30 min to write) |
| **Mutability** | Living document — evolves during implementation | Immutable once accepted; superseded by new ADR if changed |
| **Lifetime** | Temporary — serves the project | Permanent — serves the organization |

**Lukas Niessen** (The Atlantic) summarizes the flow:
```
Write RFC → Async Review (Comments) → Decision Meeting → Write ADR
```
> https://building.theatlantic.com/how-to-make-architecture-decisions-rfcs-adrs-and-getting-everyone-aligned-ab82e5384d2f

**Martin Fowler**: "Writing ADRs serves two purposes. Firstly they act as a record of decisions, allowing people months or years later to understand why the system is constructed in the way that it is. But perhaps even more important, the process of writing them forces the author to think through the decision properly."
> https://martinfowler.com/bliki/ArchitectureDecisionRecord.html

---

## 8. Key Principles Distilled

1. **Match rigor to blast radius.** Not everything needs an RFC. Small, reversible, single-team changes should be lightweight. Cross-team, irreversible, or architecturally significant changes demand full process.

2. **Write to think.** The act of writing forces clarity. "Vague ideas become concrete proposals with defined scope and tradeoffs" (PointFive). "If everyone agrees how the project should be done then writing the approach down should be a piece of cake" (Uber / Pragmatic Engineer).

3. **Decision-making by artifact, not by meeting.** RFCs reduce meeting load by making decisions visible, reviewable, and discoverable asynchronously. "The goal is to replace decision-making-by-meeting with decision-making-by-artifact" (Tuson Advisory).

4. **Separate exploration from commitment.** Use RFCs/design docs to explore. Use ADRs to record. Don't conflate the two.

5. **Make dependencies explicit.** Every plan should identify what blocks what. Use the plan to serialize shared dependencies and parallelize independent work.

6. **Define success before execution.** Include testing strategy, observability, monitoring, and explicit success criteria in the plan — not as afterthoughts.

7. **Treat plans as living documents.** Expect plans to change. Build in triggers for replanning. Preserve context when updating.

8. **Store plans where engineers work.** Put RFCs/ADRs in the code repo (e.g., `docs/rfcs/`, `docs/adr/`). Use PRs for review. "If someone is in the repo, they're one directory away from understanding why the code is the way it is" (Jono Herrington).

9. **The proposer owns persuasion.** The author of the plan is responsible for circulating it, integrating feedback, and driving it to decision. "The proposing engineer owns the RFC from draft to decision" (Eric Lubow).

10. **Confidence is a first-class concern.** Record how confident you are in decisions. Revisit when confidence thresholds are breached. "Record the confidence level of the decision... Sometimes an architecturally significant decision is made with low confidence" (Microsoft Azure Well-Architected Framework).

---

## 9. Sources & Further Reading

### RFC / Design Doc Processes
- Gergely Orosz, "Engineering Planning with RFCs, Design Documents and ADRs" — https://newsletter.pragmaticengineer.com/p/rfcs-and-design-docs
- Gergely Orosz, "Scaling Engineering Teams via RFCs: Writing Things Down" — https://blog.pragmaticengineer.com/scaling-engineering-teams-via-writing-things-down-rfcs/
- Eric Lubow, "When to Write an RFC (and When Not To)" — https://eric.lubow.org/2026/when-to-write-an-rfc-and-when-not-to/
- Eric Lubow, "Implementing an RFC Process That Engineers Don't Hate" — https://eric.lubow.org/2026/implementing-an-rfc-process-that-engineers-dont-hate/
- SPH, "Writing Effective RFCs: A Guide to Technical Decision Making" — https://sph.sh/en/posts/writing-effective-rfcs-principal-engineer-guide
- Tuson Advisory, "A Practical RFC Playbook" — https://tusonadvisory.com/when-you-cant-fit-engineering-in-one-room-anymore-a-practical-rfc-playbook/
- Ryan Madden, "Things I Learned at Google: Design Docs" — https://ryanmadden.net/things-i-learned-at-google-design-docs/
- PointFive, "Writing Technical Specifications: The Art of Tailoring RFCs" — https://www.pointfive.co/blog/writing-technical-specifications-the-art-of-tailoring-rfcs
- Vaidehi Joshi, "Planning for Change with RFCs" (Increment) — https://increment.com/planning/planning-with-requests-for-comments/

### ADRs
- Michael Nygard, "Documenting Architecture Decisions" (original 2011 post) — https://www.cognitect.com/blog/2011/11/15/documenting-architecture-decisions
- Martin Fowler, "Architecture Decision Record" — https://martinfowler.com/bliki/ArchitectureDecisionRecord.html
- ADR GitHub Organization — https://adr.github.io/
- Jono Herrington, "How to Write Architecture Decision Records That Actually Get Used" — https://www.jonoherrington.com/blog/how-to-write-adrs-that-actually-get-used
- Archyl, "Architecture Decision Records (ADR): The Complete Guide" — https://www.archyl.com/blog/architecture-decision-records-complete-guide
- AWS, "Master ADRs: Best Practices" — https://aws.amazon.com/blogs/architecture/master-architecture-decision-records-adrs-best-practices-for-effective-decision-making/
- Microsoft Azure Well-Architected Framework, "Maintain an ADR" — https://learn.microsoft.com/en-us/azure/well-architected/architect-role/architecture-decision-record
- AWS Prescriptive Guidance, "ADR Process" — https://docs.aws.amazon.com/prescriptive-guidance/latest/architectural-decision-records/adr-process.html

### Decomposition & Design Philosophy
- John Ousterhout, *A Philosophy of Software Design* (2nd ed., 2021) — https://web.stanford.edu/~ouster/cgi-bin/book.php
- John Ousterhout interview, "The Philosophy of Software Design" (The Pragmatic Engineer podcast) — https://newsletter.pragmaticengineer.com/p/the-philosophy-of-software-design
- GitHub, "Introducing Sub-Issues" — https://github.blog/engineering/architecture-optimization/introducing-sub-issues-enhancing-issue-management-on-github/

### Open-Source RFC Templates
- Rust RFC Template — https://github.com/rust-lang/rfcs/blob/master/0000-template.md
- React RFC Template — https://github.com/reactjs/rfcs/blob/main/0000-template.md

### Risk Management & Rollout
- Kathryn Tang, "Planning in Bets: Risk Mitigation at Scale" (Shopify) — https://shopify.engineering/risk-mitigation-at-scale
- Shopify, "Migrating to React Native's New Architecture" — https://shopify.engineering/react-native-new-architecture
- Valdez Ladd, "How Stripe Built Secure Unattended AI Agents" — https://medium.com/@oracle_43885/how-stripe-built-secure-unattended-ai-agents-merging-1-000-pull-requests-weekly-1ff42f3fe550
- Stripe, "Selective Test Execution at Stripe" — https://stripe.dev/blog/selective-test-execution-at-stripe-fast-ci-for-a-50m-line-ruby-monorepo

### Plan Evolution & Adaptive Planning
- PrimeLine, "How I Plan Complex Projects With Claude Code" — https://primeline.cc/blog/planning-with-claude-code
- TurinTech, "The Art and Science of a Good Plan" — https://turintech.ai/blog/the-art-and-science-of-a-good-plan-turning-requirements-into-reliable-results
- CrewAI, "Adaptive Re-planning When Task Results Deviate from Plan" — https://github.com/crewAIInc/crewAI/issues/4983
- Plandek, "Jira Planning vs Execution" — https://plandek.com/blog/jira-planning-vs-execution-how-engineering-leaders-predict-what-will-actually-ship/

### Spec-Driven & Agentic Planning
- Augment Code, "How Do Enterprise Teams Build Agentic Workflows?" — https://www.augmentcode.com/guides/how-do-enterprise-teams-build-agentic-workflows
- GitHub `awesome-copilot` breakdown-plan skill — https://github.com/github/awesome-copilot/blob/main/skills/breakdown-plan/SKILL.md
- CCPM (Claude Code Project Manager) — https://cc.deeptoai.com/docs/en/tools/ccpm-claude-code-project-manager
- tercel/spec-forge — https://github.com/tercel/spec-forge

---

*End of memo.*
