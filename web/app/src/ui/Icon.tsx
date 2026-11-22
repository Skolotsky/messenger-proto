const PATHS = {
  bell: (
    <>
      <path d="M6 16v-5a6 6 0 1 1 12 0v5l1.5 2h-15z" />
      <path d="M10 20a2 2 0 0 0 4 0" />
    </>
  ),
  back: <path d="M15 5l-7 7 7 7" />,
  chev: <path d="M6 9l6 6 6-6" />,
  chat: <path d="M4 5h16v11H9l-5 4z" />,
  pencil: <path d="M4 20l4-1 11-11-3-3L5 16z" />,
  close: <path d="M6 6l12 12M18 6L6 18" />,
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
