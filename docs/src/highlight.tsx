const GO_KW = new Set([
  "import", "func", "return", "package", "var", "const", "type", "range",
  "for", "if", "else", "go", "defer", "map", "struct", "interface", "chan",
]);

// hlGo tokenizes one line of Go into colored spans: strings, keywords, function
// calls, comments. Samples are controlled (no `//` inside strings), so a small
// regex is enough — no full parser needed. Render inside a `.win-body` for the
// token colors defined in base styles.
export function hlGo(line: string) {
  if (line.trim() === "") return " ";
  const ci = line.indexOf("//");
  const code = ci >= 0 ? line.slice(0, ci) : line;
  const comment = ci >= 0 ? line.slice(ci) : "";
  const out: unknown[] = [];
  const re = /("(?:[^"\\]|\\.)*")|([A-Za-z_][A-Za-z0-9_]*)|(\s+)|([^\sA-Za-z_"]+)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(code))) {
    if (m[1]) out.push(<span class="s">{m[1]}</span>);
    else if (m[2]) {
      if (GO_KW.has(m[2])) out.push(<span class="k">{m[2]}</span>);
      else if (code[re.lastIndex] === "(") out.push(<span class="fn">{m[2]}</span>);
      else out.push(m[2]);
    } else if (m[3]) out.push(m[3]);
    else if (m[4]) out.push(<span class="pu">{m[4]}</span>);
  }
  if (comment) out.push(<span class="cm">{comment}</span>);
  return out;
}

// codeLines renders a Go snippet as highlighted `.cl` rows (for a `.win-body`).
export function codeLines(src: string) {
  return src.split("\n").map((line) => <div class="cl">{hlGo(line)}</div>);
}

// hlYaml colors YAML: keys (`.yk`) and comments (`.cm`); values stay plain.
export function hlYaml(line: string) {
  const hi = line.indexOf("#");
  const hasComment = hi >= 0 && (hi === 0 || line[hi - 1] === " ");
  const code = hasComment ? line.slice(0, hi) : line;
  const comment = hasComment ? line.slice(hi) : "";
  const out: unknown[] = [];
  const m = code.match(/^(\s*(?:- )?)([\w.-]+)(:)(.*)$/);
  if (m) {
    out.push(m[1], <span class="yk">{m[2]}</span>, <span class="pu">{m[3]}</span>, m[4]);
  } else {
    out.push(code || " ");
  }
  if (comment) out.push(<span class="cm">{comment}</span>);
  return out;
}

export function yamlLines(src: string) {
  return src.split("\n").map((line) => <div class="cl">{hlYaml(line)}</div>);
}
