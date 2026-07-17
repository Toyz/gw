/**
 * A tiny syntax highlighter: one token engine driven by per-language rule
 * tables. A rule is [sticky regex, token class]; `tokenize` walks a line left to
 * right, emitting the first rule that matches at the cursor and accumulating
 * unmatched runs as plain text. Adding a language is just another table — no
 * per-language function, no duplicated keyword/comment plumbing.
 *
 * Token classes (.k .s .fn .cm .pu .yk) are colored by base styles' .win-body.
 */

type Rule = [re: RegExp, cls: string];

// tokenize highlights one line. Rule regexes MUST carry the sticky (y) flag so
// `exec` anchors at the cursor. Earlier rules win; put strings above comments so
// a `#`/`//` inside a string is never read as a comment.
function tokenize(line: string, rules: Rule[]): unknown[] {
  const out: unknown[] = [];
  let plain = "";
  const flush = () => {
    if (plain) {
      out.push(plain);
      plain = "";
    }
  };
  for (let i = 0; i < line.length; ) {
    let matched = false;
    for (const [re, cls] of rules) {
      re.lastIndex = i;
      const m = re.exec(line);
      if (m && m[0]) {
        flush();
        out.push(<span class={cls}>{m[0]}</span>);
        i += m[0].length;
        matched = true;
        break;
      }
    }
    if (!matched) plain += line[i++];
  }
  flush();
  return out.length ? out : [" "]; // preserve blank-line height
}

// ── Language rule tables ──

const GO: Rule[] = [
  [/\/\/.*/y, "cm"],
  [/"(?:[^"\\]|\\.)*"/y, "s"],
  [/\b(?:import|func|return|package|var|const|type|range|for|if|else|go|defer|map|struct|interface|chan)\b/y, "k"],
  [/[A-Za-z_]\w*(?=\()/y, "fn"], // identifier immediately before "(" is a call
  [/[^\s\w"]+/y, "pu"],
];

// TOML + YAML share the engine; only the key delimiter (= vs :) and section
// syntax differ. `[section]` is anchored to line start (^) so a mid-line array
// value like `["a"]` is never mistaken for a header.
const TOML: Rule[] = [
  [/^\[\[?[^\]]*\]\]?/y, "k"],
  [/(?:"[^"]*"|[\w.-]+)(?=\s*=)/y, "yk"],
  [/"(?:[^"\\]|\\.)*"/y, "s"],
  [/#.*/y, "cm"],
  [/[^\s\w".-]+/y, "pu"],
];

const YAML: Rule[] = [
  [/#.*/y, "cm"],
  [/"(?:[^"\\]|\\.)*"/y, "s"],
  [/[\w.-]+(?=:)/y, "yk"],
  [/[^\s\w".-]+/y, "pu"],
];

// ── Public: one renderer per language ──

// render splits a snippet into highlighted `.cl` rows for a `.win-body`.
function render(src: string, rules: Rule[]) {
  return src.split("\n").map((line) => <div class="cl">{tokenize(line, rules)}</div>);
}

export const codeLines = (src: string) => render(src, GO);
export const tomlLines = (src: string) => render(src, TOML);
export const yamlLines = (src: string) => render(src, YAML);
