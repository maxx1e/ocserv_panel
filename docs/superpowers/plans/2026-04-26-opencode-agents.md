# OpenCode Project Agents Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add project-scoped OpenCode primary and subagent definitions for the ocserv admin panel.

**Architecture:** Use `.opencode/agents/*.md` Markdown agent files as documented by OpenCode. Define one project primary agent for coordination and focused subagents for backend, frontend, persistence, ocserv integration, config safety, security, deployment, certificates, testing, docs, and review.

**Tech Stack:** OpenCode Markdown agent files, YAML frontmatter, repository guidance from `AGENTS.md`.

---

### Task 1: Create Project Agent Directory

**Files:**
- Create: `.opencode/agents/`

- [x] **Step 1: Create the project-scoped OpenCode agent directory**

Run: `mkdir -p .opencode/agents`

Expected: directory exists and contains project agent Markdown files after Task 2.

### Task 2: Add Primary Agent

**Files:**
- Create: `.opencode/agents/ocserv-build.md`

- [x] **Step 1: Create the project primary agent**

Add a `mode: primary` OpenCode agent that coordinates work, preserves the local-first architecture, and can invoke only the approved project subagents.

### Task 3: Add Focused Subagents

**Files:**
- Create: `.opencode/agents/backend-api.md`
- Create: `.opencode/agents/frontend-admin-ux.md`
- Create: `.opencode/agents/sqlite-persistence.md`
- Create: `.opencode/agents/ocserv-integration.md`
- Create: `.opencode/agents/config-safety.md`
- Create: `.opencode/agents/security-rbac.md`
- Create: `.opencode/agents/deployment-ops.md`
- Create: `.opencode/agents/certificate-lifecycle.md`
- Create: `.opencode/agents/test-verification.md`
- Create: `.opencode/agents/docs-adr.md`
- Create: `.opencode/agents/code-review.md`

- [x] **Step 1: Create one subagent per approved role**

Each subagent has `mode: subagent`, a focused description, permissions appropriate to its risk profile, and project-specific instructions that reinforce `AGENTS.md` constraints.

### Task 4: Verify Agent Files

**Files:**
- Read: `.opencode/agents/*.md`

- [x] **Step 1: List created agent files**

Run: `ls .opencode/agents`

Expected: all 12 Markdown files are present.

- [x] **Step 2: Read representative files**

Read `.opencode/agents/ocserv-build.md` and `.opencode/agents/code-review.md` to verify frontmatter and prompt content.

### Task 5: Add Project Skill Guidance

**Files:**
- Modify: `.opencode/agents/ocserv-build.md`
- Modify: `.opencode/agents/backend-api.md`
- Modify: `.opencode/agents/frontend-admin-ux.md`
- Modify: `.opencode/agents/sqlite-persistence.md`
- Modify: `.opencode/agents/ocserv-integration.md`
- Modify: `.opencode/agents/config-safety.md`
- Modify: `.opencode/agents/security-rbac.md`
- Modify: `.opencode/agents/deployment-ops.md`
- Modify: `.opencode/agents/certificate-lifecycle.md`
- Modify: `.opencode/agents/test-verification.md`
- Modify: `.opencode/agents/docs-adr.md`
- Modify: `.opencode/agents/code-review.md`

- [x] **Step 1: Add relevant skill mappings to each agent**

Each agent includes a `Relevant skills` section with Go, Svelte, process, test, security, or review skills appropriate to that agent's responsibility.

- [x] **Step 2: Add the invocation rule**

Each agent says: `Before work matching any listed skill, invoke that skill and follow it.`

- [x] **Step 3: Verify skill sections exist**

Run: search `.opencode/agents/*.md` for `Relevant skills:`

Expected: 12 matches, one per project agent file.
