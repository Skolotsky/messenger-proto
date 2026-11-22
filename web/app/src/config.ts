/** The organization this screen is opened for, in the address of every call (D-026). */
export const ORG: string = import.meta.env.VITE_ORG ?? 'acme';

/** There is no sign-in in the first version (D-009): this is who the app opens as. */
export const HOME_PERSON: string = import.meta.env.VITE_PERSON ?? 'marco';

/** At or below this width the app uses the phone layout. Keep equal to the CSS breakpoint. */
export const PHONE_MAX = 760;
