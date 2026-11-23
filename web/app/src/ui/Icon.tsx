const PATHS = {
  bell: (
    <>
      <path d="M6 16v-5a6 6 0 1 1 12 0v5l1.5 2h-15z" />
      <path d="M10 20a2 2 0 0 0 4 0" />
    </>
  ),
  back: <path d="M15 5l-7 7 7 7" />,
  refresh: <path d="M19 12a7 7 0 1 1-2.1-5M19 4v4h-4" />,
  chev: <path d="M6 9l6 6 6-6" />,
  chat: <path d="M4 5h16v11H9l-5 4z" />,
  pencil: <path d="M4 20l4-1 11-11-3-3L5 16z" />,
  close: <path d="M6 6l12 12M18 6L6 18" />,
  check: <path d="M5 12.5l4.5 4.5L19 7.5" />,
  pin: <path d="M7 4h10v16l-5-4-5 4z" />,
  smile: (
    <>
      <circle cx="11" cy="13" r="7" />
      <path d="M8.5 14.5a3 3 0 0 0 5 0M9 11h.01M13 11h.01M19 3v4M17 5h4" />
    </>
  ),
  rules: (
    <>
      <path d="M4 7h10M18 7h2M4 17h2M10 17h10" />
      <circle cx="16" cy="7" r="2" />
      <circle cx="8" cy="17" r="2" />
    </>
  ),
};

export type IconName = keyof typeof PATHS;

export function Icon({ name }: { name: IconName }) {
  return (
    <svg
      className="ic"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {PATHS[name]}
    </svg>
  );
}
