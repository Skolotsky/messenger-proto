/**
 * The organization this screen is opened for, in the address of every call (D-026). The entry
 * address sends the screen here with `?org=` when the current organization changes (D-033).
 */
export const ORG: string = new URLSearchParams(location.search).get('org') ?? import.meta.env.VITE_ORG ?? 'acme';

/** Answers which organization is current (`CurrentOrganization`). The demo program serves it (T-059). */
export const ENTRY_URL: string = import.meta.env.VITE_ENTRY_URL ?? '/demo/current';

/** There is no sign-in in the first version (D-009): this is who the app opens as. */
export const HOME_PERSON: string = import.meta.env.VITE_PERSON ?? 'marco';

/** At or below this width the app uses the phone layout. Keep equal to the CSS breakpoint. */
export const PHONE_MAX = 760;

/** A surface says it is behind only past this (D-007, G-019). */
export const NOTICE_AFTER_MS = 10_000;
