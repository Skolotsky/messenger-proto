---
name: visual
description: Build a self-contained HTML visual that makes complexity and choices visible to the user - system maps, flows, step-by-step walk-throughs, option comparisons, decision briefs, annotated snippets. Use whenever an explanation has structure (a flow, a boundary, a trade-off, steps over time), before asking for a decision, or when the user asks to see, show, draw or explain something.
argument-hint: "<what to make visible>"
---

# Visual

Subject: $ARGUMENTS

The user steers by qualities and contracts, not code. A visual is how they see enough of the system to decide. It replaces paragraphs; it does not decorate them. If the line above is empty, take the subject from the conversation.

## Procedure

1. **One job.** Name the single thing the reader should understand or decide after two minutes. If there are two, make two visuals.
2. **Pick the form** that shows the mechanism:
   - parts and boundaries: a diagram
   - what happens over time or under failure: steps
   - a choice: options side by side, drawing what differs
   - who does what: lanes
   - the shape of something (a payload, a contract): an annotated snippet
   - how a screen or an interaction will look and feel: a mockup the user can click through, with realistic content (inline CSS and script are fine)
3. **Build it.** Copy `.claude/skills/visual/template.html` to `docs/visuals/<slug>.html` (prefix the slug with the ID it belongs to, for example `D-004-delivery-options.html`). Replace `VISUAL TITLE` and the `<!-- CONTENT -->` line. Component markup is in `.claude/skills/visual/components.md`. Keep the template's tokens and classes so every visual reads as part of one set.
4. **Check it.** `./scripts/check` confirms it is self-contained. Look at it once rendered if the session can (browser or screenshot), at narrow width and in both themes.
5. **Show it** to the user rendered in the app, not as pasted HTML, with at most three lines of text: what it shows and what you need from them.
6. **Link it** from the research note, decision record or task it belongs to.

## Standards

- Lead with the answer or the decision needed. Depth goes into collapsed details.
- Plain language. Name things by what they do for a person, not by class or file names. Assume no knowledge of the code.
- Depict the mechanism, not the name. Label arrows with what moves or happens. Show the failure path when reliability is the point.
- Comparing options: draw the difference, mark the recommendation, state the risk of each.
- Real content from this project. Never placeholder text.
- Self-contained: no external scripts, stylesheets, fonts or images. It must open anywhere and live in git.
- Works at 400px wide and in light and dark themes. Colours come from the template tokens only.
- One visual, one claim. If a sentence says it faster, write the sentence.
