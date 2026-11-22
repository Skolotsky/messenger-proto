import { useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { newId } from '../api/door';
import type { PersonView } from '../api/types';
import type { Session } from '../state/useSession';
import { Avatar } from './Avatar';
import { ActionProblem, problemOf, type Problem } from './OwnMessage';
import { WritingIcon } from './WritingIcon';

/** A channel's name as the door accepts it: no spaces and no "#". */
export const channelName = (typed: string) =>
  typed
    .trim()
    .replace(/^#+/, '')
    .toLowerCase()
    .replace(/\s+/g, '-')
    .replace(/[^\p{L}\p{N}_-]/gu, '');

function Dialog({ label, onClose, children }: { label: string; onClose(): void; children: ReactNode }) {
  return createPortal(
    <div className="scrim center" onClick={onClose}>
      <div
        className="sheet dialog new-conv"
        role="dialog"
        aria-modal="true"
        aria-label={label}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === 'Escape') onClose();
        }}
      >
        {children}
      </div>
    </div>,
    document.body,
  );
}

function PeoplePicker({
  people,
  query,
  onQuery,
  chosen,
  onPick,
}: {
  people: PersonView[];
  query: string;
  onQuery(q: string): void;
  chosen?: Set<string>;
  onPick(p: PersonView): void;
}) {
  const q = query.trim().toLowerCase();
  const shown = q ? people.filter((p) => p.name.toLowerCase().includes(q)) : people;
  return (
    <>
      <input className="nc-input" aria-label="Find a person" placeholder="Find a person" value={query} onChange={(e) => onQuery(e.target.value)} />
      <div className="nc-people" role="group" aria-label="People">
        {shown.map((p) => (
          <button
            key={p.id}
            type="button"
            className={`opt who${chosen?.has(p.id) ? ' on' : ''}`}
            aria-pressed={chosen ? chosen.has(p.id) : undefined}
            onClick={() => onPick(p)}
          >
            <Avatar id={p.id} person={p} small />
            <b>{p.name}</b>
          </button>
        ))}
        {shown.length === 0 && <p className="sheet-note">Nobody by that name.</p>}
      </div>
    </>
  );
}

function NewChannelDialog({ session, onOpen, onClose }: { session: Session; onOpen(id: string): void; onClose(): void }) {
  const me = session.me?.id;
  const [name, setName] = useState('');
  const [query, setQuery] = useState('');
  const [chosen, setChosen] = useState<Set<string>>(new Set());
  const [problem, setProblem] = useState<Problem | null>(null);
  const [saving, setSaving] = useState(false);
  const attempt = useRef<{ body: string; id: string } | null>(null);
  const clean = channelName(name);

  const submit = async (e?: FormEvent) => {
    e?.preventDefault();
    if (!clean || saving) return;
    const people = [...chosen];
    const body = JSON.stringify([clean, [...people].sort()]);
    if (attempt.current?.body !== body) attempt.current = { body, id: newId('act') };
    setSaving(true);
    setProblem(null);
    try {
      onOpen(await session.createChannel(clean, people, attempt.current.id));
      onClose();
    } catch (err) {
      const found = problemOf(err, 'Not created');
      if (!found.canRetry) attempt.current = null;
      setProblem(found);
      setSaving(false);
    }
  };

  return (
    <Dialog label="Create a channel" onClose={onClose}>
      <h2>Create a channel</h2>
      <form onSubmit={submit}>
        <label className="nc-label">
          Name
          <input
            className="nc-input"
            autoFocus
            value={name}
            placeholder="for example launch-plans"
            onChange={(e) => {
              setName(e.target.value);
              setProblem(null);
            }}
          />
        </label>
        {name.trim() && clean !== name.trim() && <p className="sheet-note">It will be called #{clean || '…'}</p>}
        <p className="nc-label">People to add{chosen.size > 0 ? ` (${chosen.size})` : ''}</p>
        <PeoplePicker
          people={session.peopleList.filter((p) => p.id !== me)}
          query={query}
          onQuery={setQuery}
          chosen={chosen}
          onPick={(p) =>
            setChosen((old) => {
              const next = new Set(old);
              if (!next.delete(p.id)) next.add(p.id);
              return next;
            })
          }
        />
        {problem && <ActionProblem problem={problem} onRetry={() => void submit()} onDiscard={onClose} />}
        <div className="dlg-acts">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button className="btn pri" disabled={!clean || saving}>
            Create
          </button>
        </div>
      </form>
    </Dialog>
  );
}

function NewDirectDialog({ session, onOpen, onClose }: { session: Session; onOpen(id: string): void; onClose(): void }) {
  const me = session.me?.id;
  const [query, setQuery] = useState('');
  const [problem, setProblem] = useState<Problem | null>(null);
  const [saving, setSaving] = useState(false);
  const attempt = useRef<{ person: PersonView; id: string } | null>(null);

  const start = async (person: PersonView) => {
    if (saving) return;
    if (attempt.current?.person.id !== person.id) attempt.current = { person, id: newId('act') };
    setSaving(true);
    setProblem(null);
    try {
      onOpen(await session.openDirect(person.id, attempt.current.id));
      onClose();
    } catch (err) {
      const found = problemOf(err, 'Could not open the conversation');
      if (!found.canRetry) attempt.current = null;
      setProblem(found);
      setSaving(false);
    }
  };

  return (
    <Dialog label="New direct message" onClose={onClose}>
      <h2>New direct message</h2>
      <PeoplePicker people={session.peopleList.filter((p) => p.id !== me)} query={query} onQuery={setQuery} onPick={start} />
      {problem && (
        <ActionProblem
          problem={problem}
          onRetry={() => {
            if (attempt.current) void start(attempt.current.person);
          }}
          onDiscard={onClose}
        />
      )}
    </Dialog>
  );
}

/** The "+" beside Channels or Direct messages (D-009, D-024). */
export function NewConversationButton({
  kind,
  session,
  onOpen,
}: {
  kind: 'channel' | 'direct';
  session: Session;
  onOpen(id: string): void;
}) {
  const [open, setOpen] = useState(false);
  const label = kind === 'channel' ? 'Create a channel' : 'New direct message';
  useEffect(() => setOpen(false), [session.me?.id]);
  return (
    <>
      <button className="nc-add" aria-label={label} title={label} onClick={() => setOpen(true)}>
        <WritingIcon name="plus" />
      </button>
      {open &&
        (kind === 'channel' ? (
          <NewChannelDialog session={session} onOpen={onOpen} onClose={() => setOpen(false)} />
        ) : (
          <NewDirectDialog session={session} onOpen={onOpen} onClose={() => setOpen(false)} />
        ))}
    </>
  );
}
