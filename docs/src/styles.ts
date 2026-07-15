import { css } from "@toyz/loom";

// base is the shared design system applied to every routed component via
// @styles(base, ...): layout, header/footer chrome, section eyebrows, the
// window chrome (terminal + code), and Go syntax-highlight token colors.
export const base = css`
  :host {
    display: block;
  }
  .mono {
    font-family: var(--mono);
  }
  a {
    color: inherit;
    text-decoration: none;
  }
  .wrap {
    max-width: 1120px;
    margin: 0 auto;
    padding: 0 2rem;
  }
  loom-icon {
    flex-shrink: 0;
  }

  /* header */
  header {
    position: sticky;
    top: 0;
    z-index: 10;
    backdrop-filter: blur(10px);
    background: rgba(10, 12, 15, 0.72);
    border-bottom: 1px solid var(--border-soft);
  }
  .header-in {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.9rem 0;
  }
  .logo::part(anchor) {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-family: var(--mono);
    color: var(--text);
  }
  .logo loom-icon {
    color: var(--amber);
  }
  .logo b {
    font-weight: 700;
    font-size: 1.05rem;
    letter-spacing: -0.02em;
  }
  .logo .tag {
    font-family: var(--sans);
    font-weight: 400;
    font-size: 0.82rem;
    color: var(--dim);
  }
  .logo .tag::before {
    content: "/";
    margin-right: 0.5rem;
    color: var(--border);
  }
  @media (max-width: 560px) {
    .logo .tag {
      display: none;
    }
  }
  .nav {
    display: flex;
    align-items: center;
    gap: 1.6rem;
    font-size: 0.9rem;
  }
  .nav loom-link::part(anchor) {
    color: var(--dim);
    transition: color 0.15s;
  }
  .nav loom-link::part(anchor):hover {
    color: var(--text);
  }
  .nav loom-link.active::part(anchor) {
    color: var(--amber);
  }
  .gh {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    color: var(--dim);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.4rem 0.8rem;
    transition: color 0.15s, border-color 0.15s;
  }
  .gh:hover {
    color: var(--text);
    border-color: #2c343e;
  }

  /* footer */
  footer {
    border-top: 1px solid var(--border-soft);
    margin-top: 2rem;
  }
  .footer-in {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 2.5rem 0 3.5rem;
    color: var(--dim);
    font-size: 0.86rem;
    flex-wrap: wrap;
    gap: 0.75rem;
  }
  .footer-in a:hover {
    color: var(--amber);
  }

  /* sections */
  section {
    padding: 3.75rem 0;
    border-top: 1px solid var(--border-soft);
  }
  section:first-of-type {
    border-top: none;
  }
  .eyebrow {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    font-family: var(--mono);
    font-size: 0.76rem;
    text-transform: uppercase;
    letter-spacing: 0.15em;
    color: var(--dim);
    margin: 0 0 2rem;
  }
  .eyebrow loom-icon {
    color: var(--amber);
  }

  /* window chrome (terminal + code) */
  .win {
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--panel);
    overflow: hidden;
    box-shadow: 0 24px 60px -34px rgba(0, 0, 0, 0.85),
      0 -1px 0 0 rgba(255, 255, 255, 0.03) inset;
  }
  .win-bar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.7rem 1rem;
    border-bottom: 1px solid var(--border-soft);
    background: var(--panel-2);
  }
  .dot {
    width: 11px;
    height: 11px;
    border-radius: 50%;
    background: #2b323b;
  }
  .win-title {
    margin-left: 0.5rem;
    font-family: var(--mono);
    font-size: 0.76rem;
    color: var(--dim);
  }
  .win-body {
    padding: 1.15rem 1.35rem 1.4rem;
    font-family: var(--mono);
    font-size: 0.85rem;
    line-height: 1.85;
    overflow-x: auto;
  }

  /* terminal lines */
  .ln {
    white-space: pre;
  }
  .ln .p {
    color: var(--amber);
  }
  .ln.prompt {
    color: var(--text);
    margin-top: 0.55rem;
  }
  .ln.prompt:first-child {
    margin-top: 0;
  }
  .ln.add,
  .ln.ok {
    color: var(--green);
  }
  .ln.path {
    color: var(--teal);
  }
  .ln.dim {
    color: var(--dim);
  }
  .cursor {
    display: inline-block;
    width: 8px;
    height: 1em;
    background: var(--amber);
    vertical-align: text-bottom;
    animation: blink 1.1s steps(2, start) infinite;
  }
  @keyframes blink {
    50% {
      opacity: 0;
    }
  }

  /* code + Go syntax tokens */
  .code .win-body {
    line-height: 1.7;
    font-size: 0.8rem;
  }
  .cl {
    white-space: pre;
    color: #cdd5dd;
  }
  .win-body .k {
    color: var(--violet);
  }
  .win-body .s {
    color: #9ecf7f;
  }
  .win-body .fn {
    color: var(--cyan);
  }
  .win-body .cm {
    color: #61707c;
  }
  .win-body .pu {
    color: #7f8a95;
  }
  .win-body .yk {
    color: var(--teal);
  }
`;
