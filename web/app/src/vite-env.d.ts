/// <reference types="vite/client" />

interface Window {
  Theme: {
    pref(): 'system' | 'light' | 'dark';
    resolved(): 'light' | 'dark';
    set(value: 'system' | 'light' | 'dark'): void;
  };
}
