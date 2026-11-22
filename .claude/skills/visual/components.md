# Visual components

Markup for `template.html`. Copy what the visual needs and leave out the rest. Every colour comes from the tokens in the template, so use the classes and never a literal colour.

Colour roles: `you` (orange) marks where the user's attention is needed. `agent` (blue) marks the agent's work. `ok` (green) marks what is recommended, holds or is verified. `risk` (red) marks risk, breakage or what is not verified.

## Header (always)

```html
<header class="top">
  <div class="label">Decision brief · D-004 · 2026-11-22</div>
  <h1>Title that names the subject</h1>
  <p class="lede">The one-sentence takeaway.</p>
</header>
```

## Needs you (when the page asks the user for something; place it right after the header)

```html
<div class="ask">
  <div class="label">Needs you</div>
  <p><strong>The one question.</strong> The recommendation and its reason, in a sentence.</p>
</div>
```

## Section

```html
<section>
  <h2>What this part shows</h2>
  <p>One or two sentences, then a component.</p>
</section>
```

## Options (a choice: draw what differs, mark the recommendation, state each risk)

```html
<div class="options">
  <div class="option pick">
    <header><h3>A. Name of the option</h3><span class="chip ok">Recommended</span></header>
    <p>What it is, in one or two sentences.</p>
    <dl>
      <dt>Upside</dt><dd>The main gain.</dd>
      <dt>Risk</dt><dd>What can go wrong, and how we would notice.</dd>
      <dt>Undo</dt><dd>Cheap, moderate or expensive to reverse.</dd>
    </dl>
  </div>
  <div class="option">
    <header><h3>B. Name of the option</h3></header>
    <p>...</p>
    <dl><dt>Upside</dt><dd>...</dd><dt>Risk</dt><dd>...</dd><dt>Undo</dt><dd>...</dd></dl>
  </div>
</div>
```

## Steps (what happens in order: a request, a failure and its recovery, a user journey)

```html
<ol class="steps">
  <li><div class="body"><h3>What happens first</h3><p>Plain description. Add a snippet when the shape of the data matters.</p></div></li>
  <li><div class="body"><h3>What happens next</h3><p>...</p></div></li>
</ol>
```

## Lanes (who does what, row by row; a tinted "you" cell means the user acts, an `idle` one means they do not)

The optional `say` line is something the user might say in their own words. Never show a slash command.

```html
<div class="lanes">
  <div class="head first">Phase</div><div class="head you">You</div><div class="head agent">Agent</div>

  <div class="phase"><strong>Decide</strong><span class="say">“What are my options?”</span></div>
  <div class="you">Make the call.</div>
  <div class="agent">Prepares the brief and records the choice.</div>

  <div class="phase"><strong>Implement</strong></div>
  <div class="you idle">Nothing needed.</div>
  <div class="agent">Builds the task.</div>
</div>
```

## Diagram (parts, boundaries and what moves between them; inline SVG only)

Size by `viewBox`. Text 11 to 13px. Label every arrow with what moves or happens. Use the box classes for meaning: `box`, `box-you`, `box-agent`, `box-ok`, `box-risk`. Show the failure path when reliability is the point.

```html
<figure class="diagram">
  <div class="scroll">
    <svg viewBox="0 0 680 120" role="img" aria-label="The same claim as the caption">
      <defs>
        <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
          <path d="M0 0 L10 5 L0 10 z" fill="currentColor"/>
        </marker>
      </defs>
      <rect class="box" x="10" y="30" width="150" height="56" rx="6"/>
      <text class="t" x="85" y="54" text-anchor="middle">Service A</text>
      <text class="t2" x="85" y="72" text-anchor="middle">what it owns</text>
      <line class="edge" x1="160" y1="58" x2="330" y2="58" marker-end="url(#arrow)"/>
      <text class="edge-label" x="245" y="50" text-anchor="middle">publishes OrderPlaced</text>
      <rect class="box-agent" x="330" y="30" width="150" height="56" rx="6"/>
      <text class="t" x="405" y="62" text-anchor="middle">Service B</text>
    </svg>
  </div>
  <figcaption>One sentence stating what the picture shows.</figcaption>
</figure>
```

## Snippet (the shape of something: a payload, a contract, a config; highlight the part that matters)

```html
<figure class="snippet">
  <pre><code>{
  "id": "evt_01",
  <mark>"idempotency_key": "msg_42:v3"</mark>
}</code></pre>
  <figcaption>What the highlighted line is for.</figcaption>
</figure>
```

## Table (comparisons, guardrail lists, findings)

```html
<div class="table-wrap">
  <table>
    <thead><tr><th>Guardrail</th><th>Enforced by</th><th>Status</th></tr></thead>
    <tbody>
      <tr><td>The sentence.</td><td>The check.</td><td><span class="chip ok">Enforced</span></td></tr>
    </tbody>
  </table>
</div>
```

## Panels (two or three groups side by side)

```html
<div class="split">
  <div class="panel you"><div class="label">You decide</div><ul><li>...</li></ul></div>
  <div class="panel agent"><div class="label">Agent decides</div><ul><li>...</li></ul></div>
</div>
```

## Details (depth the user can open if interested; keep the essentials outside)

```html
<details>
  <summary>How the numbers were measured</summary>
  <div class="stack"><p>...</p></div>
</details>
```

## Chips and notes

```html
<span class="chip you">Your call</span> <span class="chip agent">Agent</span>
<span class="chip ok">Verified</span> <span class="chip risk">Not verified</span>
<p class="note">A quieter line for caveats and pointers.</p>
```

## Footer (always: what this belongs to and where the full reasoning lives)

```html
<footer class="foot">
  <span>Belongs to D-004 · Evidence: R-002, E-001</span>
  <span>Full reasoning: <code>docs/decisions/D-004-example.md</code></span>
</footer>
```
