// The contract's types, from the generated file. Do not redefine them here.
export type {
  ActionSaved,
  Bucket,
  ChannelDial,
  ConversationPart,
  ConversationSummary,
  DialLevel,
  FeedCounts,
  FeedPart,
  FeedRow,
  FocusRule,
  HistoryAnswer,
  HistoryEntry,
  HistoryTab,
  KeptBy,
  Level,
  LiveOut,
  PeopleAction,
  PersonView,
  PreviewAnswer,
  Reaction,
  ReplyInProgress,
  RowAct,
  RowChange,
  ScreenFeedAnswer,
  SettingsAnswer,
  ScreenSearchAnswer,
  SearchResult,
  StartAnswer,
  SuggestAnswer,
  ThreadSummary,
} from '../../../../contracts/generated/types';

import type { Level, Reaction, ReplyInProgress, ThreadSummary } from '../../../../contracts/generated/types';

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
  in_progress?: ReplyInProgress[];
  has_after?: boolean;
}
