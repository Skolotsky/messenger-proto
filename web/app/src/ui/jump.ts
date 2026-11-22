/** A search hit to open: in its conversation, in its thread when it is a reply (T-038, point 22). */
export interface Jump {
  conversation: string;
  thread: string | null;
  message: string;
  number: number;
  /** Opening the same hit again scrolls to it again. */
  nonce: number;
}
