import type { ActivityMarks } from '../state/useFeed';

interface Props {
  marks: ActivityMarks;
  /** Keep the space when there is no mark, so that a row of tabs does not move. */
  reserve?: boolean;
}

/** Focus and Needs you, each with its number; under Focus only the second is a quiet dot (D-037). */
export function ActivityMark({ marks, reserve }: Props) {
  const { focus, needs, needsQuiet } = marks;
  if (!focus && !needs && !reserve) return null;
  return (
    <span className="marks">
      {focus > 0 && <span className="badge m-focus" title={`${focus} unread in Focus`}>{focus}</span>}
      {needs > 0 &&
        (needsQuiet ? (
          <span className="dot" role="img" aria-label="Unread in Needs you" title="Unread in Needs you" />
        ) : (
          <span className="badge m-needs" title={`${needs} unread in Needs you`}>{needs}</span>
        ))}
    </span>
  );
}
