---
name: experiment
description: Experiment phase. Run a small, time-boxed spike that settles one uncertain technical or product question with evidence, in an isolated folder under experiments/. Use when research leaves a question only trying it can answer, or when the user asks to try, prototype, measure or compare something.
---

# Experiment

Question: $ARGUMENTS

An experiment buys evidence for a decision. It is throwaway code with an honest conclusion. If the question above is empty, take it from the conversation.

## Procedure

1. **Design before code.** In `experiments/E-nnn-<slug>/README.md` (from `.claude/skills/experiment/experiment-template.md`) write the question, the decision it feeds, the hypothesis, the method, and the result that would change our mind. If no result could change the decision, skip the experiment and say so.
2. **Scope.** The smallest setup that answers the question. State the time box. Add a board item (`T-nnn` `experiment`) under Now.
3. **Run it.** Keep everything inside the experiment folder. Product code never imports from `experiments/`. Record the exact commands and the raw results.
4. **Conclude.** Fill in result and conclusion, including what the experiment does not show (scale, environment, shortcuts taken). A negative result is a result.
5. **Hand off.** Tell the user the answer, how sure we are, and what it means for the decision. Make a visual when the result is a comparison or a behaviour over time. Move the board item to `tasks/done.md`.

## Standards

- Measure, do not assume. Numbers come with the conditions they were measured under.
- Reproducible: one command reruns it.
- A spike does not become the product. Once the question is answered, the code stays behind.
