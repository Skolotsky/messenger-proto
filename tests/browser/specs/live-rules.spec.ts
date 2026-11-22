import { expect, test } from '@playwright/test';
import { FIRST_STEP_MS, judge, LAST_STEP_MS, missedSomething, nextStep, reconnectPause, takePiece } from '../../../web/app/src/api/stream';

/** A small seeded generator, so that the variation in the pause is the same on every run. */
function seeded(seed: number) {
  let x = seed >>> 0 || 1;
  return () => {
    x ^= x << 13;
    x ^= x >>> 17;
    x ^= x << 5;
    x >>>= 0;
    return x / 2 ** 32;
  };
}

test('an update is applied only directly after the last one applied', () => {
  expect(judge(10, 10, 14)).toBe('apply');
  expect(judge(10, 12, 14), 'one in between is missing').toBe('ask');
  expect(judge(14, 10, 14), 'already held').toBe('ignore');
  expect(judge(20, 10, 14), 'older than what is held').toBe('ignore');
});

test('last_sent past our number means something was lost', () => {
  expect(missedSomething(10, 12)).toBe(true);
  expect(missedSomething(12, 12)).toBe(false);
  expect(missedSomething(14, 12), 'it was sent before our last update').toBe(false);
});

test('the reconnect pause doubles to 1 s, varies, and never exceeds its step', () => {
  const steps: number[] = [];
  for (let s = FIRST_STEP_MS, i = 0; i < 6; i++, s = nextStep(s)) steps.push(s);
  expect(steps).toEqual([100, 200, 400, 800, 1000, 1000]);

  const random = seeded(7);
  for (const step of steps) {
    const pauses = Array.from({ length: 200 }, () => reconnectPause(step, random));
    expect(Math.max(...pauses), 'never above the step').toBeLessThanOrEqual(step);
    expect(Math.min(...pauses), 'never immediate').toBeGreaterThanOrEqual(step / 2);
    expect(new Set(pauses).size, 'screens do not all come back in the same instant').toBeGreaterThan(100);
  }
  expect(reconnectPause(LAST_STEP_MS, () => 1)).toBe(LAST_STEP_MS);
  expect(reconnectPause(LAST_STEP_MS, () => 5), 'a bad random source cannot pass the ceiling').toBe(LAST_STEP_MS);
});

test('pieces of a reply join by code point, and a gap is a gap', () => {
  const held = { at: 2, text: '😀a' };
  expect(takePiece(held, 2, 'bc')).toEqual({ at: 4, text: '😀abc' });
  expect(takePiece(held, 1, 'ab'), 'the overlap is not repeated').toEqual({ at: 3, text: '😀ab' });
  expect(takePiece(held, 0, '😀'), 'already held').toBe(held);
  expect(takePiece(held, 3, 'x')).toBe('gap');
});
