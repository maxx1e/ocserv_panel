<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from '@codemirror/view';
  import { EditorState } from '@codemirror/state';
  import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
  import { searchKeymap, highlightSelectionMatches } from '@codemirror/search';
  import {
    createExtensions,
    readOnlyCompartment,
    setIssuesEffect,
    setGutterIssuesEffect,
  } from './config-editor-extensions';
  import type { ConfigIssue } from '$lib/api';

  let {
    content = $bindable(''),
    readonly = false,
    issues = [] as ConfigIssue[],
    class: className = '',
  }: {
    content: string;
    readonly?: boolean;
    issues?: ConfigIssue[];
    class?: string;
  } = $props();

  let container: HTMLDivElement;
  let view: EditorView | null = $state(null);
  let internalUpdate = false;

  onMount(() => {
    const state = EditorState.create({
      doc: content,
      extensions: [
        lineNumbers(),
        highlightActiveLine(),
        highlightActiveLineGutter(),
        history(),
        highlightSelectionMatches(),
        keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap]),
        ...createExtensions(readonly),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            internalUpdate = true;
            content = update.state.doc.toString();
            internalUpdate = false;
          }
        }),
      ],
    });

    view = new EditorView({ state, parent: container });
  });

  onDestroy(() => {
    view?.destroy();
    view = null;
  });

  // Sync external content changes into the editor
  $effect(() => {
    if (!view || internalUpdate) return;
    const currentDoc = view.state.doc.toString();
    if (content !== currentDoc) {
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: content },
      });
    }
  });

  // Sync readonly state
  $effect(() => {
    if (!view) return;
    view.dispatch({
      effects: readOnlyCompartment.reconfigure(EditorView.editable.of(!readonly)),
    });
  });

  // Sync issues into decorations and gutter
  $effect(() => {
    if (!view) return;
    view.dispatch({
      effects: [
        setIssuesEffect.of(issues),
        setGutterIssuesEffect.of(issues),
      ],
    });
  });

  // Public method: scroll to a specific line and highlight it
  export function scrollToLine(line: number) {
    if (!view) return;
    const lineCount = view.state.doc.lines;
    const targetLine = Math.max(1, Math.min(line, lineCount));
    const lineInfo = view.state.doc.line(targetLine);
    view.dispatch({
      selection: { anchor: lineInfo.from },
      effects: EditorView.scrollIntoView(lineInfo.from, { y: 'center' }),
    });
    view.focus();
  }
</script>

<div bind:this={container} class="border border-slate-200 rounded-lg overflow-hidden {className}"></div>
