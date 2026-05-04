import '@testing-library/jest-dom/vitest';

// The sessions page calls navigator.clipboard.writeText() in the copy button.
// Neither jsdom nor happy-dom implement the Clipboard API, so stub it globally.
Object.defineProperty(navigator, 'clipboard', {
  value: { writeText: vi.fn().mockResolvedValue(undefined) },
  writable: true
});
