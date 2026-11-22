import { Icon } from './Icon';

/**
 * Seam for T-055: the feed, its buckets, the history tabs and the arrivals control belong here.
 * The header already holds the title and the place beside it that arrivals will use (D-023).
 */
export function Activity({ orgName, onClose }: { orgName: string; onClose(): void }) {
  return (
    <>
      <header className="top">
        <h1>
          <small className="org-line">{orgName}</small>
          Activity
        </h1>
        <button className="icon-btn act-close" onClick={onClose} aria-label="Close Activity" title="Close Activity">
          <Icon name="close" />
        </button>
      </header>
      <div className="feed-wrap">
        <div className="feed-scroll">
          <div className="empty start">
            <p>
              <b>The activity feed is not built yet.</b>
              <br />
              Channels and direct messages work now.
            </p>
          </div>
        </div>
      </div>
    </>
  );
}
