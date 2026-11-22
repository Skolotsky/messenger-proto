import { useEffect, useState, useSyncExternalStore } from 'react';
import { NOTICE_AFTER_MS, PHONE_MAX } from '../config';

const phoneQuery = `(max-width:${PHONE_MAX}px)`;

export function useIsPhone(): boolean {
  return useSyncExternalStore(
    (notify) => {
      const mq = matchMedia(phoneQuery);
      mq.addEventListener('change', notify);
      return () => mq.removeEventListener('change', notify);
    },
    () => matchMedia(phoneQuery).matches,
  );
}

const isLooking = () => document.visibilityState === 'visible' && document.hasFocus();

/** True only while this window is in front of the person: nothing is read otherwise (D-015). */
export function useIsLooking(): boolean {
  const [looking, setLooking] = useState(isLooking);
  useEffect(() => {
    const update = () => setLooking(isLooking());
    window.addEventListener('focus', update);
    window.addEventListener('blur', update);
    document.addEventListener('visibilitychange', update);
    update();
    return () => {
      window.removeEventListener('focus', update);
      window.removeEventListener('blur', update);
      document.removeEventListener('visibilitychange', update);
    };
  }, []);
  return looking;
}

export function useThemePref() {
  const pref = useSyncExternalStore(
    (notify) => {
      window.addEventListener('themechange', notify);
      return () => window.removeEventListener('themechange', notify);
    },
    () => window.Theme.pref(),
  );
  return [pref, window.Theme.set] as const;
}

/** A per-device convenience. It may be unavailable, so every access is guarded. */
export const stored = {
  get(key: string): string | null {
    try {
      return localStorage.getItem(key);
    } catch {
      return null;
    }
  },
  set(key: string, value: string | null) {
    try {
      if (value === null) localStorage.removeItem(key);
      else localStorage.setItem(key, value);
    } catch {
      /* the app works without it */
    }
  },
};

/** Since when the connection is lost, once that is long enough to say so (D-007, G-019). */
export function useOfflineNotice(since: number | null): number | null {
  const [due, setDue] = useState(false);
  useEffect(() => {
    setDue(false);
    if (since === null) return;
    const timer = setTimeout(() => setDue(true), Math.max(0, since + NOTICE_AFTER_MS - Date.now()));
    return () => clearTimeout(timer);
  }, [since]);
  return due ? since : null;
}
