const PATHS = {
  trash: <path d="M5 7h14M10 7V5h4v2M7 7l1 12h8l1-12" />,
  unflag: (
    <>
      <path d="M6 21V4h11l-2 4 2 4H6" />
      <path d="M3 3l18 18" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="6" />
      <path d="M16 16l4 4" />
    </>
  ),
  plus: <path d="M12 5v14M5 12h14" />,
};

export function WritingIcon({ name }: { name: keyof typeof PATHS }) {
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
