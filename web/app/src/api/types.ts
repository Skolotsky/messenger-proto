// The contract's types, from the generated file. Do not redefine them here.
export type {
  ActionSaved,
  ConversationPart,
  ConversationSummary,
  Level,
  LiveOut,
  PeopleAction,
  PersonView,
  Reaction,
  StartAnswer,
  ThreadSummary,
} from '../../../../contracts/generated/types';

import type { Level, Reaction, ThreadSummary } from '../../../../contracts/generated/types';

/**
 * The generated `Message` is an open map, because the generator cannot express the schema's
 * `allOf` rules for deleted and stopped messages. These are the schema's required fields.
 */
export interface ChatMessage {
  id: string;
  number: number;
  version: number;
  conversation: string;
  thread: string | null;
  author: string;
  author_is_agent: boolean;
  participants: string[] | null;
  text: string;
  mentions: string[];
  level: Level | null;
  reactions: Reaction[];
  stopped: boolean;
  created_at: string;
  edited_at: string | null;
  deleted: boolean;
}

export interface MessageRow {
  message: ChatMessage;
  thread_summary: ThreadSummary | null;
}

export interface MessagesPage {
  number: number;
  messages: MessageRow[];
}
