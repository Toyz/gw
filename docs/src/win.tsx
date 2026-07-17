import { LoomElement, component, prop, reactive, styles, css } from "@toyz/loom";
import { clipboard } from "@toyz/loom/element";
import { base } from "./styles";
import { codeLines, tomlLines, yamlLines } from "./highlight";

const copyStyles = css`
  :host {
    display: inline-flex;
  }
  .wc {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 22px;
    background: none;
    border: 1px solid transparent;
    border-radius: 6px;
    color: var(--dim, #8b949e);
    cursor: pointer;
    transition: color 0.15s, background 0.15s, border-color 0.15s;
  }
  .wc:hover {
    color: var(--text, #e8edf2);
    background: rgba(255, 255, 255, 0.04);
    border-color: var(--border, #212832);
  }
  .wc.ok {
    color: var(--green, #5fcf90);
  }
`;

// <gw-copy text="…"> — a clipboard button (Loom @clipboard) that flips to a
// check for a beat after copying.
@component("gw-copy")
@styles(copyStyles)
export class GwCopy extends LoomElement {
  @prop accessor text = "";
  @reactive accessor copied = false;

  @clipboard("write")
  copy() {
    this.copied = true;
    setTimeout(() => (this.copied = false), 1400);
    return this.text;
  }

  update() {
    return (
      <button
        class={this.copied ? "wc ok" : "wc"}
        onClick={() => this.copy()}
        aria-label={this.copied ? "Copied" : "Copy"}
      >
        <loom-icon name={this.copied ? "check" : "copy"} size={13} />
      </button>
    );
  }
}

// promptLine renders a "$ …" command row, dimming any trailing `# comment`.
function promptLine(text: string) {
  const hi = text.indexOf("#");
  const cmd = hi >= 0 ? text.slice(0, hi) : text;
  const comment = hi >= 0 ? text.slice(hi) : "";
  return (
    <div class="ln prompt">
      <span class="p">$ </span>
      {cmd}
      {comment ? <span class="cm">{comment}</span> : ""}
    </div>
  );
}

// bar renders a window's title bar: traffic-light dots, title, and a copy button
// (right-aligned via base's `.win-bar gw-copy` rule).
function bar(title: string, copyText: string) {
  return (
    <div class="win-bar">
      <span class="dot" />
      <span class="dot" />
      <span class="dot" />
      <span class="win-title">{title}</span>
      <gw-copy text={copyText} />
    </div>
  );
}

// TermLine is one row of a <gw-term>: a status class (prompt/ok/fail/dim/path/…)
// and its text. "prompt" gets the "$ " marker.
export interface TermLine {
  c: string;
  t: string;
}

// cmdLines turns a "$ …"-prefixed multi-line string into prompt TermLines — the
// common shape for a scaffold/command terminal.
export function cmdLines(src: string): TermLine[] {
  return src.split("\n").map((l) => ({ c: "prompt", t: l.replace(/^\$ /, "") }));
}

// <gw-term title="…" lines={[...]}> — a terminal window rendered from line data.
// The chrome, `.ln` line styles, and token colors all live in base, adopted into
// this element's shadow, so callers just pass data.
@component("gw-term")
@styles(base)
export class GwTerm extends LoomElement {
  @prop accessor title = "";
  @prop accessor lines: TermLine[] = [];

  update() {
    return (
      <div class="win">
        {bar(this.title, this.lines.map((l) => l.t).join("\n"))}
        <div class="win-body">
          {this.lines.map((l) =>
            l.c === "prompt" ? promptLine(l.t) : <div class={"ln " + l.c}>{l.t}</div>,
          )}
        </div>
      </div>
    );
  }
}

// <gw-code title="…" lang="go|toml|yaml" src="…"> — a code window highlighted
// internally, so a snippet is one self-contained tag.
const HIGHLIGHT: Record<string, (src: string) => unknown> = {
  go: codeLines,
  toml: tomlLines,
  yaml: yamlLines,
};

@component("gw-code")
@styles(base)
export class GwCode extends LoomElement {
  @prop accessor title = "";
  @prop accessor lang = "go";
  @prop accessor src = "";

  update() {
    const highlight = HIGHLIGHT[this.lang] ?? codeLines;
    return (
      <div class="win code">
        {bar(this.title, this.src)}
        <div class="win-body">{highlight(this.src)}</div>
      </div>
    );
  }
}
