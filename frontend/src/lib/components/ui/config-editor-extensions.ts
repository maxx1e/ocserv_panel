import {
  EditorView,
  Decoration,
  type DecorationSet,
  gutter,
  GutterMarker,
} from '@codemirror/view';
import { StateEffect, StateField, RangeSet, Compartment } from '@codemirror/state';
import { StreamLanguage } from '@codemirror/language';
import type { ConfigIssue } from '$lib/api';

// ─── INI-like syntax for ocserv config ────────────────────────

export const ocservConfigLanguage = StreamLanguage.define({
  token(stream) {
    // Comments
    if (stream.sol() && stream.peek() === '#') {
      stream.skipToEnd();
      return 'comment';
    }
    // At start of line, match the key (before =)
    if (stream.sol()) {
      if (stream.match(/^[a-zA-Z][a-zA-Z0-9_-]*/)) {
        return 'keyword';
      }
    }
    // Operator
    if (stream.match(/^\s*=\s*/)) {
      return 'operator';
    }
    // Rest of line is value
    stream.next();
    return 'string';
  },
  startState() {
    return {};
  },
});

// ─── Issue line highlights ────────────────────────────────────

export const setIssuesEffect = StateEffect.define<ConfigIssue[]>();

const errorLineDeco = Decoration.line({ class: 'cm-line-error' });
const warningLineDeco = Decoration.line({ class: 'cm-line-warning' });
const infoLineDeco = Decoration.line({ class: 'cm-line-info' });

export const issueLineDecorations = StateField.define<DecorationSet>({
  create() {
    return Decoration.none;
  },
  update(decos, tr) {
    for (const effect of tr.effects) {
      if (effect.is(setIssuesEffect)) {
        const issues = effect.value;
        const builder: { from: number; deco: Decoration }[] = [];
        for (const issue of issues) {
          if (!issue.line || issue.line < 1 || issue.line > tr.state.doc.lines) continue;
          const lineInfo = tr.state.doc.line(issue.line);
          const deco =
            issue.severity === 'error'
              ? errorLineDeco
              : issue.severity === 'warning'
                ? warningLineDeco
                : infoLineDeco;
          builder.push({ from: lineInfo.from, deco });
        }
        // Sort by position and deduplicate
        builder.sort((a, b) => a.from - b.from);
        const seen = new Set<number>();
        const unique = builder.filter((b) => {
          if (seen.has(b.from)) return false;
          seen.add(b.from);
          return true;
        });
        return Decoration.set(unique.map((b) => b.deco.range(b.from)));
      }
    }
    return decos;
  },
  provide: (f) => EditorView.decorations.from(f),
});

// ─── Severity gutter markers ─────────────────────────────────

class IssueMark extends GutterMarker {
  severity: string;
  constructor(severity: string) {
    super();
    this.severity = severity;
  }
  toDOM() {
    const span = document.createElement('span');
    span.className =
      this.severity === 'error'
        ? 'cm-gutter-error'
        : this.severity === 'warning'
          ? 'cm-gutter-warning'
          : 'cm-gutter-info';
    span.textContent =
      this.severity === 'error' ? '●' : this.severity === 'warning' ? '▲' : 'ℹ';
    return span;
  }
}

export const setGutterIssuesEffect = StateEffect.define<ConfigIssue[]>();

const issueGutterState = StateField.define<Map<number, string>>({
  create() {
    return new Map();
  },
  update(map, tr) {
    for (const effect of tr.effects) {
      if (effect.is(setGutterIssuesEffect)) {
        const newMap = new Map<number, string>();
        for (const issue of effect.value) {
          if (!issue.line || issue.line < 1) continue;
          const existing = newMap.get(issue.line);
          // Error takes priority over warning over info
          if (
            !existing ||
            issue.severity === 'error' ||
            (issue.severity === 'warning' && existing === 'info')
          ) {
            newMap.set(issue.line, issue.severity);
          }
        }
        return newMap;
      }
    }
    return map;
  },
});

export const issueGutter = gutter({
  class: 'cm-issue-gutter',
  markers(view) {
    const map = view.state.field(issueGutterState);
    const ranges: Array<ReturnType<typeof IssueMark.prototype.range>> = [];
    for (const [lineNum, severity] of map) {
      if (lineNum <= view.state.doc.lines) {
        const line = view.state.doc.line(lineNum);
        ranges.push(new IssueMark(severity).range(line.from));
      }
    }
    return RangeSet.of(ranges, true);
  },
});

// ─── Read-only compartment ───────────────────────────────────

export const readOnlyCompartment = new Compartment();

// ─── Theme ───────────────────────────────────────────────────

export const ocservTheme = EditorView.theme({
  '&': {
    fontSize: '13px',
    fontFamily: 'ui-monospace, SFMono-Regular, Consolas, "Liberation Mono", Menlo, monospace',
  },
  '&.cm-focused': {
    outline: '2px solid #3b82f6',
    outlineOffset: '-1px',
    borderRadius: '0.5rem',
  },
  '.cm-scroller': {
    overflow: 'auto',
    minHeight: '400px',
  },
  '.cm-content': {
    padding: '12px 0',
  },
  '.cm-gutters': {
    backgroundColor: '#f8fafc',
    borderRight: '1px solid #e2e8f0',
    color: '#64748b',
  },
  '.cm-activeLineGutter': {
    backgroundColor: '#f1f5f9',
  },
  '.cm-activeLine': {
    backgroundColor: '#f8fafc',
  },
  '.cm-selectionBackground': {
    backgroundColor: '#e2e8f0 !important',
  },
  '.cm-line-error': {
    backgroundColor: 'rgba(254, 202, 202, 0.3)',
  },
  '.cm-line-warning': {
    backgroundColor: 'rgba(254, 240, 138, 0.3)',
  },
  '.cm-line-info': {
    backgroundColor: 'rgba(191, 219, 254, 0.2)',
  },
  '.cm-issue-gutter': {
    width: '18px',
    paddingLeft: '3px',
  },
  '.cm-gutter-error': {
    color: '#ef4444',
    fontSize: '11px',
    lineHeight: '1.5',
  },
  '.cm-gutter-warning': {
    color: '#f59e0b',
    fontSize: '11px',
    lineHeight: '1.5',
  },
  '.cm-gutter-info': {
    color: '#3b82f6',
    fontSize: '11px',
    lineHeight: '1.5',
  },
});

// ─── Extension bundle ────────────────────────────────────────

export function createExtensions(readonly: boolean) {
  return [
    ocservConfigLanguage,
    issueLineDecorations,
    issueGutterState,
    issueGutter,
    ocservTheme,
    readOnlyCompartment.of(EditorView.editable.of(!readonly)),
  ];
}
