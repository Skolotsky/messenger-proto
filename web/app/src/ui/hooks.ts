import { useEffect, useState, useSyncExternalStore } from 'react';
import { PHONE_MAX } from '../config';

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
