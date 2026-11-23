import type { PersonView } from '../api/types';
import type { Session } from '../state/useSession';

/** What an agent is doing now, beside its name. The latest status counts (agent-door.md). */
export function agentStateText(person: PersonView | undefined, session: Session): string | null {
  const status = person?.is_agent ? person.status : null;
  if (!status) return null;
  if (status.state === 'working') return 'Working';
  if (status.state === 'done') return 'Done';
  if (!status.on) return 'Waiting';
  if (status.on === session.me?.id) return 'Waiting on you';
  const name = session.people.get(status.on)?.name ?? status.on;
  return `Waiting on ${name.split(' ')[0]}`;
}

export function AgentState({ person, session }: { person: PersonView | undefined; session: Session }) {
  const text = agentStateText(person, session);
  if (!text || !person?.status) return null;
  return (
    <span className={`agent-state s-${person.status.state}`} data-agent-state={person.id}>
      <span className="st-dot" aria-hidden="true" />
      {text}
    </span>
  );
}
