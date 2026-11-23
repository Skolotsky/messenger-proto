interface Props {
  attention: number;
  quietUnread: boolean;
  /** Keep the space when there is no mark, so that a row of tabs does not move. */
  reserve?: boolean;
}

/** The one mark on Activity: a number for what calls you, a dot for what can wait (D-036). */
export function ActivityMark({ attention, quietUnread, reserve }: Props) {
  if (attention > 0) return <span className="badge">{attention}</span>;
  if (quietUnread) return <span className="dot" role="img" aria-label="Unread in Needs you" title="Unread in Needs you" />;
  return reserve ? <span className="badge-slot" aria-hidden="true" /> : null;
}
