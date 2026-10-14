import { afterEach } from "vitest";
import { cleanup, configure } from "@testing-library/react";

afterEach(() => cleanup());

// The first test in a file also loads its lazy routes; 1 s (the default) is too tight on slow CI runners.
configure({ asyncUtilTimeout: 5000 });

// Node's Request rejects relative URLs; the browser resolves them against the page. Match the browser.
const NodeRequest = globalThis.Request;
globalThis.Request = class extends NodeRequest {
  constructor(input: RequestInfo | URL, init?: RequestInit) {
    super(typeof input === "string" ? new URL(input, window.location.href) : input, init);
  }
} as typeof Request;

// jsdom has <dialog> but not showModal/close; enough of them for BottomSheet.
HTMLDialogElement.prototype.showModal ??= function (this: HTMLDialogElement) {
  this.open = true;
};
HTMLDialogElement.prototype.close ??= function (this: HTMLDialogElement) {
  this.open = false;
};
