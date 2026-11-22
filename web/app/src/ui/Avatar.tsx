import type { CSSProperties } from 'react';
import type { PersonView } from '../api/types';

function hue(id: string): number {
  let h = 0;
  for (const c of id) h = (h * 31 + c.codePointAt(0)!) % 360;
  return h;
}

function initials(name: string, isAgent: boolean): string {
  const words = name.trim().split(/[\s-]+/).filter(Boolean);
  if (words.length === 0) return '?';
  if (isAgent) return Array.from(words[0]).slice(0, 2).join('');
  const first = Array.from(words[0])[0];
  const second = words[1] ? Array.from(words[1])[0] : '';
  return `${first}${second}`.toUpperCase();
}

/** A person's square avatar. An agent carries a small robot on its corner (D-024). */
export function Avatar({ id, person, small }: { id: string; person?: PersonView; small?: boolean }) {
  const isAgent = person?.is_agent ?? false;
  const name = person?.name ?? id;
  return (
    <span
      className={`av${isAgent ? ' ag' : ''}${small ? ' xs' : ''}`}
      style={{ '--h': hue(id) } as CSSProperties}
      aria-hidden="true"
    >
      {initials(name, isAgent)}
      {isAgent && (
        <i className="bot" title={`${name}: an agent, not a person`}>
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <rect x="5" y="9" width="14" height="10" rx="2.5" />
            <path d="M12 9V5.5M9.2 13.5h.01M14.8 13.5h.01" />
            <circle cx="12" cy="4.2" r="1.2" />
          </svg>
        </i>
      )}
    </span>
  );
}
