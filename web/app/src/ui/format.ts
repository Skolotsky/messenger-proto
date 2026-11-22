const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

export const dayKey = (iso: string) => startOfDay(new Date(iso));

/** The label on a line between days: Today, Yesterday, a weekday within the week, else a date. */
export function dayLabel(iso: string, now = new Date()): string {
  const day = new Date(iso);
  const days = Math.round((startOfDay(now) - startOfDay(day)) / 86_400_000);
  if (days === 0) return 'Today';
  if (days === 1) return 'Yesterday';
  if (days > 1 && days < 7) return day.toLocaleDateString('en-GB', { weekday: 'long' });
  const sameYear = day.getFullYear() === now.getFullYear();
  return day.toLocaleDateString('en-GB', { day: 'numeric', month: 'long', ...(sameYear ? {} : { year: 'numeric' }) });
}

export const timeOf = (iso: string) =>
  new Date(iso).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });

/** "Today 10:42", "Yesterday 16:05", for the link under a message. */
export function whenOf(iso: string, now = new Date()): string {
  return `${dayLabel(iso, now)} ${timeOf(iso)}`;
}

/** The time at the right of a feed row: the hour today, otherwise the day. */
export function rowTime(iso: string, now = new Date()): string {
  return dayKey(iso) === dayKey(now.toISOString()) ? timeOf(iso) : dayLabel(iso, now);
}
