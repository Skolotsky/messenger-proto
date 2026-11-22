/**
 * The rules of the numbered live streams (conversations-to-screen.md), with nothing else in them,
 * so that they can be tested on their own.
 */

export type Verdict = 'apply' | 'ignore' | 'ask';

/** An update is applied only directly after the last number applied. */
export function judge(last: number, after: number, number: number): Verdict {
  if (number <= last) return 'ignore';
  return after === last ? 'apply' : 'ask';
}

/** The regular `last_sent`: a number past ours means something was lost on the way. */
export const missedSomething = (last: number, sent: number) => sent > last;

export const FIRST_STEP_MS = 100;
export const LAST_STEP_MS = 1000;

export const nextStep = (step: number) => Math.min(step * 2, LAST_STEP_MS);

/**
 * The pause before reconnecting: between half the step and the step, so that a whole
 * organization does not reconnect in the same instant, and never longer than the step.
 */
export function reconnectPause(step: number, random: () => number): number {
  const r = Math.min(Math.max(random(), 0), 1);
  return step / 2 + (step / 2) * r;
}

export interface HeldText {
  at: number;
  text: string;
}

/**
 * A piece of a reply in progress. `at` and lengths count code points. A piece that starts past
 * what is held is a gap, for which the text so far is fetched.
 */
export function takePiece(held: HeldText, at: number, text: string): HeldText | 'gap' {
  if (at > held.at) return 'gap';
  const piece = Array.from(text);
  const end = at + piece.length;
  if (end <= held.at) return held;
  return { at: end, text: held.text + piece.slice(held.at - at).join('') };
}
