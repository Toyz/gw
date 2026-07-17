import { LoomAttribute, attribute, reactive, prop, on, styles, css } from "@toyz/loom";

type Placement = "auto" | "top" | "bottom";

// Teach JSX about the custom `tip` attribute: a bare string, or an object with
// a forced placement — <code tip="…"> / <code tip={{ text, placement: "top" }}>.
declare module "@toyz/loom/jsx-runtime" {
  interface LoomCustomAttributes {
    tip?: string | { text: string; placement?: Placement };
  }
}

const tipStyles = css`
  .tip {
    position: fixed;
    z-index: 200;
    max-width: 280px;
    transform: translateX(-50%);
    padding: 0.5rem 0.75rem;
    border-radius: 8px;
    background: var(--panel-2, #14181e);
    border: 1px solid var(--border, #212832);
    box-shadow: 0 18px 44px -22px rgba(0, 0, 0, 0.85),
      0 -1px 0 0 rgba(255, 255, 255, 0.03) inset;
    color: var(--text, #e8edf2);
    font-family: var(--mono, monospace);
    font-size: 0.75rem;
    line-height: 1.5;
    pointer-events: none;
    animation: tipIn 0.12s ease;
  }
  .tip.top {
    transform: translateX(-50%) translateY(-100%);
  }
  /* A rotated square straddling the bubble edge — two borders dropped so the
     outer point keeps the bubble's outline and the base blends into the fill. */
  .tip-arrow {
    position: absolute;
    left: calc(50% + var(--dx, 0px));
    width: 9px;
    height: 9px;
    background: var(--panel-2, #14181e);
    border: 1px solid var(--border, #212832);
    transform: translateX(-50%) rotate(45deg);
  }
  .tip.bottom .tip-arrow {
    top: -5px;
    border-right: 0;
    border-bottom: 0;
  }
  .tip.top .tip-arrow {
    bottom: -5px;
    border-left: 0;
    border-top: 0;
  }
  @keyframes tipIn {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
`;

// tip is a Loom @attribute controller: any element with `tip="…"` shows a
// hover/focus tooltip bubble portaled to <body>. @on binds host + window events,
// @prop takes structured args, @styles scopes the portal, and update() flips the
// bubble above/below and clamps it to the viewport each render.
@attribute("tip")
@styles(tipStyles)
export class Tip extends LoomAttribute {
  @prop accessor text = ""; // set from object arg: tip={{ text }}
  @prop accessor placement: Placement = "auto";
  @reactive accessor shown = false;

  @on("mouseenter")
  @on("focusin")
  show() {
    this.shown = true;
  }

  @on("mouseleave")
  @on("focusout")
  hide() {
    this.shown = false;
  }

  // Keep the bubble glued to its anchor while open — page scroll / resize moves
  // the host, so re-render to recompute the fixed-position coordinates.
  @on(window, "scroll")
  @on(window, "resize")
  reflow() {
    if (this.shown) this.rerender();
  }

  update() {
    const label = this.text || this.value;
    if (!this.shown || !label) return;

    const r = this.el.getBoundingClientRect();
    const margin = 8;
    const estHeight = 44; // enough-room heuristic for the flip
    const halfWidth = 130; // ~ max-width / 2, for horizontal clamping

    const anchorX = r.left + r.width / 2;
    let side = this.placement;
    if (side === "auto") {
      side = r.bottom + margin + estHeight > window.innerHeight ? "top" : "bottom";
    }
    const y = side === "top" ? r.top - margin : r.bottom + margin;
    const x = Math.min(
      Math.max(anchorX, halfWidth + margin),
      window.innerWidth - halfWidth - margin,
    );
    const dx = anchorX - x; // shift the caret back onto the anchor after clamping

    return (
      <div class={`tip ${side}`} style={`left:${x}px;top:${y}px;--dx:${dx}px`}>
        {label}
        <span class="tip-arrow" />
      </div>
    );
  }
}
