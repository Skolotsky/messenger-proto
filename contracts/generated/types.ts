// Generated from contracts/schema/contexia.json. Do not edit: run ./scripts/contracts

/**
 * From (inclusive) and to (exclusive), in Unicode code points.
 *
 * @minItems 2
 *
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Span".
 */
export type Span = never[];
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Level".
 */
export type Level = "flag" | "info";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Bucket".
 */
export type Bucket = "focus" | "needs_you" | "fyi";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "DialLevel".
 */
export type DialLevel = "focus" | "all" | "default" | "muted" | "needs_you" | "fyi" | "off" | "auto";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChannelDial".
 */
export type ChannelDial = "focus" | "all" | "default" | "muted";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ItemKind".
 */
export type ItemKind = "flags" | "mentions" | "direct_messages";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Stream".
 */
export type Stream = "conversations" | "feed";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NoteNow".
 */
export type NoteNow = "read" | "deleted";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "WithdrawReason".
 */
export type WithdrawReason = "read" | "deleted" | "fenced" | "rule_changed";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RuleType".
 */
export type RuleType = "person" | "conversation" | "item_kind" | "text";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "KeptBy".
 */
export type KeptBy = "flag" | "you" | "both";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowState".
 */
export type RowState = "none" | "changed" | "retracted";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "HistoryTab".
 */
export type HistoryTab = "direct" | "mentions" | "threads" | "reactions";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ConversationType".
 */
export type ConversationType = "channel" | "direct";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Mode".
 */
export type Mode = "running" | "slow" | "off";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ReceiverState".
 */
export type ReceiverState = "running" | "down" | "paused" | "stopped";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ReceiverName".
 */
export type ReceiverName = "search" | "feed" | "sender";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NoteState".
 */
export type NoteState = "held" | "sent" | "withdrawn" | "history";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentState".
 */
export type AgentState = "working" | "waiting" | "done";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ScriptCommand".
 */
export type ScriptCommand = "start" | "pause" | "pace" | "play_situation";
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FocusRule".
 */
export type FocusRule = {
  [k: string]: unknown;
};
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Message".
 */
export type Message = {
  [k: string]: unknown;
};
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderDial".
 */
export type ChangeReaderDial = {
  [k: string]: unknown;
};
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Change".
 */
export type Change =
  | ChangeMessageCreated
  | ChangeMessageChanged
  | ChangeMessageDeleted
  | ChangeReplyStarted
  | ChangeReplyAbandoned
  | ChangeReactionAdded
  | ChangeReactionRemoved
  | ChangePersonJoined
  | ChangeChannelCreated
  | ChangeChannelPeopleAdded
  | ChangeDirectCreated
  | ChangeReaderRead
  | ChangeReaderWatchingStarted
  | ChangeReaderWatchingEnded
  | ChangeReaderKeep
  | ChangeReaderDone
  | ChangeReaderDial
  | ChangeReaderFocusRule
  | ChangeReaderFocusPerson
  | ChangeReaderFocusAutoAdd
  | ChangeReaderFocusOnly
  | ChangeReaderReleaseSetting
  | ChangeReaderPreference
  | ChangeReaderProfileLine
  | ChangeTick;
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionDial".
 */
export type ActionDial = {
  [k: string]: unknown;
};
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PeopleAction".
 */
export type PeopleAction =
  | ActionSend
  | ActionEdit
  | ActionDelete
  | ActionCreateChannel
  | ActionAddPeople
  | ActionOpenDirect
  | ActionReact
  | ActionRead
  | ActionKeep
  | ActionDone
  | ActionDial
  | ActionFocusRule
  | ActionFocusPerson
  | ActionFocusAutoAdd
  | ActionFocusOnly
  | ActionReleaseSetting
  | ActionPreference
  | ActionProfileLine;
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowActDial".
 */
export type RowActDial = {
  [k: string]: unknown;
};
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowAct".
 */
export type RowAct = RowActDial | RowActFocusPerson | RowActFocusRule | RowActDone;
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LevelEntry".
 */
export type LevelEntry = {
  [k: string]: unknown;
};
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LiveIn".
 */
export type LiveIn = LiveResume | LiveWatching;
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ConversationPart".
 */
export type ConversationPart =
  | PartMessageCreated
  | PartMessageChanged
  | PartMessageDeleted
  | PartReactionChanged
  | PartThreadSummary
  | PartUnread
  | PartReadMoved
  | PartReplyStarted
  | PartReplyAbandoned
  | PartConversationAdded
  | PartPeopleAdded
  | PartPersonAdded
  | PartPreference
  | PartProfileLine;
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FeedPart".
 */
export type FeedPart =
  FeedPartRowChanged | FeedPartRowRemoved | FeedPartCounts | FeedPartHistoryChanged | FeedPartSettingsChanged;
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LiveOut".
 */
export type LiveOut =
  LiveFeedUpdate | LiveConversationsUpdate | LiveLastSent | LiveTooFarBehind | LiveReset | LivePiece | LiveAgentStatus;
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchChange".
 */
export type SearchChange = SearchCreated | SearchChanged | SearchDeletedChange;
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Note".
 */
export type Note = NoteDue | NoteChanged | NoteWithdrawn | NoteClock;
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentEvent".
 */
export type AgentEvent =
  | AgentEventMentioned
  | AgentEventDirectMessage
  | AgentEventReply
  | AgentEventReaction
  | AgentEventChanged
  | AgentEventDeleted;

/**
 * Every payload of docs/contracts/. The words there are the authority; this file is the exact form.
 */
export interface ContractTypes {
  Span?: Span;
  Level?: Level;
  Bucket?: Bucket;
  DialLevel?: DialLevel;
  ChannelDial?: ChannelDial;
  ItemKind?: ItemKind;
  Stream?: Stream;
  NoteNow?: NoteNow;
  WithdrawReason?: WithdrawReason;
  RuleType?: RuleType;
  KeptBy?: KeptBy;
  RowState?: RowState;
  HistoryTab?: HistoryTab;
  ConversationType?: ConversationType;
  Mode?: Mode;
  ReceiverState?: ReceiverState;
  ReceiverName?: ReceiverName;
  NoteState?: NoteState;
  AgentState?: AgentState;
  ScriptCommand?: ScriptCommand;
  RowKey?: RowKey;
  Reaction?: Reaction;
  FocusRule?: FocusRule;
  Message?: Message;
  DeletedMessage?: DeletedMessage;
  ThreadSummary?: ThreadSummary;
  MessageView?: MessageView;
  PersonStatus?: PersonStatus;
  PersonView?: PersonView;
  Preference?: Preference;
  ConversationSummary?: ConversationSummary;
  ChangeMessageCreated?: ChangeMessageCreated;
  ChangeMessageChanged?: ChangeMessageChanged;
  ChangeMessageDeleted?: ChangeMessageDeleted;
  ChangeReplyStarted?: ChangeReplyStarted;
  ChangeReplyAbandoned?: ChangeReplyAbandoned;
  ChangeReactionAdded?: ChangeReactionAdded;
  ChangeReactionRemoved?: ChangeReactionRemoved;
  ChangePersonJoined?: ChangePersonJoined;
  ChangeChannelCreated?: ChangeChannelCreated;
  ChangeChannelPeopleAdded?: ChangeChannelPeopleAdded;
  ChangeDirectCreated?: ChangeDirectCreated;
  ChangeReaderRead?: ChangeReaderRead;
  ChangeReaderWatchingStarted?: ChangeReaderWatchingStarted;
  ChangeReaderWatchingEnded?: ChangeReaderWatchingEnded;
  ChangeReaderKeep?: ChangeReaderKeep;
  ChangeReaderDone?: ChangeReaderDone;
  ChangeReaderDial?: ChangeReaderDial;
  ChangeReaderFocusRule?: ChangeReaderFocusRule;
  ChangeReaderFocusPerson?: ChangeReaderFocusPerson;
  ChangeReaderFocusAutoAdd?: ChangeReaderFocusAutoAdd;
  ChangeReaderFocusOnly?: ChangeReaderFocusOnly;
  ChangeReaderReleaseSetting?: ChangeReaderReleaseSetting;
  ChangeReaderPreference?: ChangeReaderPreference;
  ChangeReaderProfileLine?: ChangeReaderProfileLine;
  ChangeTick?: ChangeTick;
  Change?: Change;
  ChangeBatch?: ChangeBatch;
  ChangeApplied?: ChangeApplied;
  IAmAt?: IAmAt;
  StaleReset?: StaleReset;
  OrganizationStopped?: OrganizationStopped;
  NewMessage?: NewMessage;
  ActionSend?: ActionSend;
  ActionEdit?: ActionEdit;
  ActionDelete?: ActionDelete;
  ActionCreateChannel?: ActionCreateChannel;
  ActionAddPeople?: ActionAddPeople;
  ActionOpenDirect?: ActionOpenDirect;
  ActionReact?: ActionReact;
  ActionRead?: ActionRead;
  ActionKeep?: ActionKeep;
  ActionDone?: ActionDone;
  ActionDial?: ActionDial;
  ActionFocusRule?: ActionFocusRule;
  ActionFocusPerson?: ActionFocusPerson;
  ActionFocusAutoAdd?: ActionFocusAutoAdd;
  ActionFocusOnly?: ActionFocusOnly;
  ActionReleaseSetting?: ActionReleaseSetting;
  ActionPreference?: ActionPreference;
  ActionProfileLine?: ActionProfileLine;
  PeopleAction?: PeopleAction;
  ActionSaved?: ActionSaved;
  Refusal?: Refusal;
  StartAnswer?: StartAnswer;
  ReplyInProgress?: ReplyInProgress;
  MessagesAnswer?: MessagesAnswer;
  RowActDial?: RowActDial;
  RowActFocusPerson?: RowActFocusPerson;
  RowActFocusRule?: RowActFocusRule;
  RowActDone?: RowActDone;
  RowAct?: RowAct;
  RowChange?: RowChange;
  RowExplanation?: RowExplanation;
  FeedRow?: FeedRow;
  FeedCounts?: FeedCounts;
  FeedRowsAnswer?: FeedRowsAnswer;
  ScreenFeedAnswer?: ScreenFeedAnswer;
  HistoryEntry?: HistoryEntry;
  HistoryAnswer?: HistoryAnswer;
  FocusPerson?: FocusPerson;
  LevelEntry?: LevelEntry;
  SettingsAnswer?: SettingsAnswer;
  PreviewCounts?: PreviewCounts;
  PreviewAnswer?: PreviewAnswer;
  NotAvailable?: NotAvailable;
  SuggestAnswer?: SuggestAnswer;
  SearchResult?: SearchResult;
  SearchAnswer?: SearchAnswer;
  ScreenSearchAnswer?: ScreenSearchAnswer;
  ReplyText?: ReplyText;
  ResumeNumbers?: ResumeNumbers;
  LiveResume?: LiveResume;
  WatchingNote?: WatchingNote;
  LiveWatching?: LiveWatching;
  LiveIn?: LiveIn;
  PartMessageCreated?: PartMessageCreated;
  PartMessageChanged?: PartMessageChanged;
  PartMessageDeleted?: PartMessageDeleted;
  PartReactionChanged?: PartReactionChanged;
  PartThreadSummary?: PartThreadSummary;
  PartUnread?: PartUnread;
  PartReadMoved?: PartReadMoved;
  PartReplyStarted?: PartReplyStarted;
  PartReplyAbandoned?: PartReplyAbandoned;
  PartConversationAdded?: PartConversationAdded;
  PartPeopleAdded?: PartPeopleAdded;
  PartPersonAdded?: PartPersonAdded;
  PartPreference?: PartPreference;
  PartProfileLine?: PartProfileLine;
  ConversationPart?: ConversationPart;
  FeedPartRowChanged?: FeedPartRowChanged;
  FeedPartRowRemoved?: FeedPartRowRemoved;
  FeedPartCounts?: FeedPartCounts;
  FeedPartHistoryChanged?: FeedPartHistoryChanged;
  FeedPartSettingsChanged?: FeedPartSettingsChanged;
  FeedPart?: FeedPart;
  LiveConversationsUpdate?: LiveConversationsUpdate;
  LiveFeedUpdate?: LiveFeedUpdate;
  LastSentNumbers?: LastSentNumbers;
  LiveLastSent?: LiveLastSent;
  LiveTooFarBehind?: LiveTooFarBehind;
  LiveReset?: LiveReset;
  PieceBody?: PieceBody;
  LivePiece?: LivePiece;
  AgentStatusBody?: AgentStatusBody;
  LiveAgentStatus?: LiveAgentStatus;
  LiveOut?: LiveOut;
  RowUpdate?: RowUpdate;
  RowUpdatesPage?: RowUpdatesPage;
  NewEdition?: NewEdition;
  SearchMessage?: SearchMessage;
  SearchDeleted?: SearchDeleted;
  SearchCreated?: SearchCreated;
  SearchChanged?: SearchChanged;
  SearchDeletedChange?: SearchDeletedChange;
  SearchChange?: SearchChange;
  SearchApplied?: SearchApplied;
  SearchRefusal?: SearchRefusal;
  NoteDue?: NoteDue;
  NoteChanged?: NoteChanged;
  NoteWithdrawn?: NoteWithdrawn;
  NoteClock?: NoteClock;
  Note?: Note;
  NoteTaken?: NoteTaken;
  NoteBehind?: NoteBehind;
  NotificationEntry?: NotificationEntry;
  AgentEventMentioned?: AgentEventMentioned;
  AgentEventDirectMessage?: AgentEventDirectMessage;
  AgentEventReply?: AgentEventReply;
  AgentEventChanged?: AgentEventChanged;
  AgentEventDeleted?: AgentEventDeleted;
  AgentEventReaction?: AgentEventReaction;
  AgentEvent?: AgentEvent;
  AgentEvents?: AgentEvents;
  TooFarBehind?: TooFarBehind;
  AgentPost?: AgentPost;
  AgentPosted?: AgentPosted;
  AgentReplyStart?: AgentReplyStart;
  AgentPiece?: AgentPiece;
  AgentReplyFinish?: AgentReplyFinish;
  PieceMissing?: PieceMissing;
  AgentStatus?: AgentStatus;
  AgentReaction?: AgentReaction;
  AgentDirect?: AgentDirect;
  AgentDirectAnswer?: AgentDirectAnswer;
  FeedPlace?: FeedPlace;
  SearchPlace?: SearchPlace;
  SenderPlace?: SenderPlace;
  ReceiverStatus?: ReceiverStatus;
  DeliveryStatus?: DeliveryStatus;
  ParkedChange?: ParkedChange;
  ParkedList?: ParkedList;
  ModeBody?: ModeBody;
  RebuildRequest?: RebuildRequest;
  RestartRequest?: RestartRequest;
  NotificationList?: NotificationList;
  PeopleList?: PeopleList;
  ScriptStep?: ScriptStep;
  Situation?: Situation;
  ScriptState?: ScriptState;
  ScriptRequest?: ScriptRequest;
  SeedOpened?: SeedOpened;
  SeedBatch?: SeedBatch;
  SeedThrough?: SeedThrough;
  Empty?: Empty;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowKey".
 */
export interface RowKey {
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Reaction".
 */
export interface Reaction {
  emoji: string;
  people: string[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "DeletedMessage".
 */
export interface DeletedMessage {
  id: string;
  version: number;
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ThreadSummary".
 */
export interface ThreadSummary {
  replies: number;
  last_reply_at: string | null;
  people: string[];
  unread: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "MessageView".
 */
export interface MessageView {
  message: Message;
  thread_summary: ThreadSummary | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PersonStatus".
 */
export interface PersonStatus {
  state: AgentState;
  on: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PersonView".
 */
export interface PersonView {
  id: string;
  name: string;
  is_agent: boolean;
  line: string | null;
  status: PersonStatus | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Preference".
 */
export interface Preference {
  key: string;
  value: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ConversationSummary".
 */
export interface ConversationSummary {
  conversation: string;
  type: ConversationType;
  name: string;
  people: string[];
  unread: boolean;
  dial: ChannelDial;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeMessageCreated".
 */
export interface ChangeMessageCreated {
  kind: "message.created";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  message: Message;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeMessageChanged".
 */
export interface ChangeMessageChanged {
  kind: "message.changed";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  message: Message;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeMessageDeleted".
 */
export interface ChangeMessageDeleted {
  kind: "message.deleted";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  message: DeletedMessage;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReplyStarted".
 */
export interface ChangeReplyStarted {
  kind: "reply.started";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  message: string;
  conversation: string;
  thread: string | null;
  author: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReplyAbandoned".
 */
export interface ChangeReplyAbandoned {
  kind: "reply.abandoned";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  message: string;
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReactionAdded".
 */
export interface ChangeReactionAdded {
  kind: "reaction.added";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  emoji: string;
  message: string;
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReactionRemoved".
 */
export interface ChangeReactionRemoved {
  kind: "reaction.removed";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  emoji: string;
  message: string;
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangePersonJoined".
 */
export interface ChangePersonJoined {
  kind: "person.joined";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  name: string;
  is_agent: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeChannelCreated".
 */
export interface ChangeChannelCreated {
  kind: "channel.created";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  conversation: string;
  name: string;
  people: string[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeChannelPeopleAdded".
 */
export interface ChangeChannelPeopleAdded {
  kind: "channel.people_added";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  conversation: string;
  people: string[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeDirectCreated".
 */
export interface ChangeDirectCreated {
  kind: "direct.created";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  conversation: string;
  people: string[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderRead".
 */
export interface ChangeReaderRead {
  kind: "reader.read";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  conversation: string;
  thread: string | null;
  up_to: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderWatchingStarted".
 */
export interface ChangeReaderWatchingStarted {
  kind: "reader.watching_started";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  conversation: string;
  thread: string | null;
  connection: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderWatchingEnded".
 */
export interface ChangeReaderWatchingEnded {
  kind: "reader.watching_ended";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  conversation: string;
  thread: string | null;
  connection: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderKeep".
 */
export interface ChangeReaderKeep {
  kind: "reader.keep";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderDone".
 */
export interface ChangeReaderDone {
  kind: "reader.done";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderFocusRule".
 */
export interface ChangeReaderFocusRule {
  kind: "reader.focus_rule";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  rule: FocusRule;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderFocusPerson".
 */
export interface ChangeReaderFocusPerson {
  kind: "reader.focus_person";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  about: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderFocusAutoAdd".
 */
export interface ChangeReaderFocusAutoAdd {
  kind: "reader.focus_auto_add";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderFocusOnly".
 */
export interface ChangeReaderFocusOnly {
  kind: "reader.focus_only";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderReleaseSetting".
 */
export interface ChangeReaderReleaseSetting {
  kind: "reader.release_setting";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderPreference".
 */
export interface ChangeReaderPreference {
  kind: "reader.preference";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  key: string;
  value: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeReaderProfileLine".
 */
export interface ChangeReaderProfileLine {
  kind: "reader.profile_line";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
  person: string;
  text: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeTick".
 */
export interface ChangeTick {
  kind: "tick";
  org: string;
  reset: number;
  number: number;
  previous: number;
  at: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeBatch".
 */
export interface ChangeBatch {
  /**
   * @maxItems 100
   */
  changes: Change[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ChangeApplied".
 */
export interface ChangeApplied {
  applied: boolean;
  already?: boolean;
  fenced?: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "IAmAt".
 */
export interface IAmAt {
  i_am_at: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "StaleReset".
 */
export interface StaleReset {
  stale_reset: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "OrganizationStopped".
 */
export interface OrganizationStopped {
  organization_stopped: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NewMessage".
 */
export interface NewMessage {
  id: string;
  conversation: string;
  thread: string | null;
  text: string;
  mentions: string[];
  level: Level | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionSend".
 */
export interface ActionSend {
  kind: "send";
  id: string;
  as: string;
  message: NewMessage;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionEdit".
 */
export interface ActionEdit {
  kind: "edit";
  id: string;
  as: string;
  message: string;
  text: string;
  mentions: string[];
  level: Level | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionDelete".
 */
export interface ActionDelete {
  kind: "delete";
  id: string;
  as: string;
  message: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionCreateChannel".
 */
export interface ActionCreateChannel {
  kind: "create_channel";
  id: string;
  as: string;
  name: string;
  people: string[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionAddPeople".
 */
export interface ActionAddPeople {
  kind: "add_people";
  id: string;
  as: string;
  conversation: string;
  people: string[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionOpenDirect".
 */
export interface ActionOpenDirect {
  kind: "open_direct";
  id: string;
  as: string;
  with: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionReact".
 */
export interface ActionReact {
  kind: "react";
  id: string;
  as: string;
  message: string;
  emoji: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionRead".
 */
export interface ActionRead {
  kind: "read";
  id: string;
  as: string;
  conversation: string;
  thread: string | null;
  up_to: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionKeep".
 */
export interface ActionKeep {
  kind: "keep";
  id: string;
  as: string;
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionDone".
 */
export interface ActionDone {
  kind: "done";
  id: string;
  as: string;
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionFocusRule".
 */
export interface ActionFocusRule {
  kind: "focus_rule";
  id: string;
  as: string;
  rule: FocusRule;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionFocusPerson".
 */
export interface ActionFocusPerson {
  kind: "focus_person";
  id: string;
  as: string;
  about: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionFocusAutoAdd".
 */
export interface ActionFocusAutoAdd {
  kind: "focus_auto_add";
  id: string;
  as: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionFocusOnly".
 */
export interface ActionFocusOnly {
  kind: "focus_only";
  id: string;
  as: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionReleaseSetting".
 */
export interface ActionReleaseSetting {
  kind: "release_setting";
  id: string;
  as: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionPreference".
 */
export interface ActionPreference {
  kind: "preference";
  id: string;
  as: string;
  key: string;
  value: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionProfileLine".
 */
export interface ActionProfileLine {
  kind: "profile_line";
  id: string;
  as: string;
  text: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ActionSaved".
 */
export interface ActionSaved {
  saved: boolean;
  number: number | null;
  conversation?: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Refusal".
 */
export interface Refusal {
  refused: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "StartAnswer".
 */
export interface StartAnswer {
  org: string;
  org_name: string;
  person: PersonView;
  people: PersonView[];
  preferences: Preference[];
  conversations: ConversationSummary[];
  reset: number;
  number: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ReplyInProgress".
 */
export interface ReplyInProgress {
  message: string;
  conversation: string;
  thread: string | null;
  author: string;
  at: number;
  text: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "MessagesAnswer".
 */
export interface MessagesAnswer {
  number: number;
  messages: MessageView[];
  in_progress: ReplyInProgress[];
  read_up_to: number | null;
  has_before: boolean;
  has_after: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowActFocusPerson".
 */
export interface RowActFocusPerson {
  kind: "focus_person";
  about: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowActFocusRule".
 */
export interface RowActFocusRule {
  kind: "focus_rule";
  rule: FocusRule;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowActDone".
 */
export interface RowActDone {
  kind: "done";
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowChange".
 */
export interface RowChange {
  label: string;
  do: RowAct;
  undo: RowAct | null;
  moved_last_7_days: number | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowExplanation".
 */
export interface RowExplanation {
  what: string;
  decided_by: string;
  otherwise: string;
  change: RowChange | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FeedRow".
 */
export interface FeedRow {
  conversation: string;
  thread: string | null;
  message: string;
  bucket: Bucket;
  unread: boolean;
  kept: KeptBy | null;
  why: string;
  newest: number;
  first_new: number | null;
  author: string;
  preview: string;
  at: string;
  new_items: number;
  reactions: Reaction[];
  state: RowState;
  fenced: boolean;
  explanation: RowExplanation;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FeedCounts".
 */
export interface FeedCounts {
  focus: number;
  needs_you: number;
  fyi: boolean;
  kept: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FeedRowsAnswer".
 */
export interface FeedRowsAnswer {
  reset: number;
  edition: number;
  through: number;
  number: number;
  rows: FeedRow[];
  counts: FeedCounts;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ScreenFeedAnswer".
 */
export interface ScreenFeedAnswer {
  reset: number;
  edition: number | null;
  through: number | null;
  number: number | null;
  rows: FeedRow[];
  counts: FeedCounts | null;
  notice: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "HistoryEntry".
 */
export interface HistoryEntry {
  message: string;
  conversation: string;
  thread: string | null;
  author: string;
  preview: string;
  at: string;
  number: number;
  unread: boolean;
  by: string | null;
  emoji: string | null;
  level: DialLevel | null;
  kept: KeptBy | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "HistoryAnswer".
 */
export interface HistoryAnswer {
  tab: HistoryTab;
  entries: HistoryEntry[];
  unread: number;
  next: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FocusPerson".
 */
export interface FocusPerson {
  person: string;
  items_last_7_days: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SettingsAnswer".
 */
export interface SettingsAnswer {
  focus_people: FocusPerson[];
  removed_people: string[];
  auto_add: boolean;
  focus_only: boolean;
  release_on_act: boolean;
  rules: FocusRule[];
  removed_rules: FocusRule[];
  levels: LevelEntry[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PreviewCounts".
 */
export interface PreviewCounts {
  moved: number;
  from: Bucket | null;
  to: Bucket | null;
  stay: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PreviewAnswer".
 */
export interface PreviewAnswer {
  last_7_days: PreviewCounts;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NotAvailable".
 */
export interface NotAvailable {
  not_available: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SuggestAnswer".
 */
export interface SuggestAnswer {
  people: PersonView[];
  everyone: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchResult".
 */
export interface SearchResult {
  message: string;
  number: number;
  created_at: string;
  version: number;
  conversation: string;
  thread: string | null;
  author: string;
  edited: boolean;
  text: string;
  match: Span[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchAnswer".
 */
export interface SearchAnswer {
  through: number;
  total: number;
  next: string | null;
  results: SearchResult[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ScreenSearchAnswer".
 */
export interface ScreenSearchAnswer {
  answer: SearchAnswer | null;
  notice: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ReplyText".
 */
export interface ReplyText {
  message: string;
  conversation: string;
  thread: string | null;
  author: string;
  at: number;
  text: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ResumeNumbers".
 */
export interface ResumeNumbers {
  reset: number;
  conversations: number;
  feed: number | null;
  feed_edition: number | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LiveResume".
 */
export interface LiveResume {
  resume: ResumeNumbers;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "WatchingNote".
 */
export interface WatchingNote {
  conversation: string;
  thread: string | null;
  open: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LiveWatching".
 */
export interface LiveWatching {
  watching: WatchingNote;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartMessageCreated".
 */
export interface PartMessageCreated {
  kind: "message.created";
  message: Message;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartMessageChanged".
 */
export interface PartMessageChanged {
  kind: "message.changed";
  message: Message;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartMessageDeleted".
 */
export interface PartMessageDeleted {
  kind: "message.deleted";
  message: DeletedMessage;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartReactionChanged".
 */
export interface PartReactionChanged {
  kind: "reaction.changed";
  message: string;
  conversation: string;
  thread: string | null;
  reactions: Reaction[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartThreadSummary".
 */
export interface PartThreadSummary {
  kind: "thread.summary";
  conversation: string;
  thread: string;
  replies: number;
  last_reply_at: string | null;
  people: string[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartUnread".
 */
export interface PartUnread {
  kind: "unread";
  conversation: string;
  thread: string | null;
  unread: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartReadMoved".
 */
export interface PartReadMoved {
  kind: "read.moved";
  conversation: string;
  thread: string | null;
  up_to: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartReplyStarted".
 */
export interface PartReplyStarted {
  kind: "reply.started";
  message: string;
  conversation: string;
  thread: string | null;
  author: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartReplyAbandoned".
 */
export interface PartReplyAbandoned {
  kind: "reply.abandoned";
  message: string;
  conversation: string;
  thread: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartConversationAdded".
 */
export interface PartConversationAdded {
  kind: "conversation.added";
  conversation_summary: ConversationSummary;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartPeopleAdded".
 */
export interface PartPeopleAdded {
  kind: "people.added";
  conversation: string;
  people: string[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartPersonAdded".
 */
export interface PartPersonAdded {
  kind: "person.added";
  person_view: PersonView;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartPreference".
 */
export interface PartPreference {
  kind: "preference";
  key: string;
  value: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PartProfileLine".
 */
export interface PartProfileLine {
  kind: "profile_line";
  person: string;
  text: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FeedPartRowChanged".
 */
export interface FeedPartRowChanged {
  kind: "row.changed";
  row: FeedRow;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FeedPartRowRemoved".
 */
export interface FeedPartRowRemoved {
  kind: "row.removed";
  row: RowKey;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FeedPartCounts".
 */
export interface FeedPartCounts {
  kind: "counts";
  counts: FeedCounts;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FeedPartHistoryChanged".
 */
export interface FeedPartHistoryChanged {
  kind: "history.changed";
  tab: HistoryTab;
  unread: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FeedPartSettingsChanged".
 */
export interface FeedPartSettingsChanged {
  kind: "settings.changed";
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LiveConversationsUpdate".
 */
export interface LiveConversationsUpdate {
  stream: "conversations";
  number: number;
  after: number;
  parts: ConversationPart[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LiveFeedUpdate".
 */
export interface LiveFeedUpdate {
  stream: "feed";
  number: number;
  after: number;
  edition: number;
  parts: FeedPart[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LastSentNumbers".
 */
export interface LastSentNumbers {
  conversations: number;
  feed: number | null;
  feed_edition: number | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LiveLastSent".
 */
export interface LiveLastSent {
  last_sent: LastSentNumbers;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LiveTooFarBehind".
 */
export interface LiveTooFarBehind {
  too_far_behind: Stream;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LiveReset".
 */
export interface LiveReset {
  reset: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PieceBody".
 */
export interface PieceBody {
  message: string;
  at: number;
  text: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LivePiece".
 */
export interface LivePiece {
  piece: PieceBody;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentStatusBody".
 */
export interface AgentStatusBody {
  agent: string;
  state: AgentState;
  on: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "LiveAgentStatus".
 */
export interface LiveAgentStatus {
  agent_status: AgentStatusBody;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowUpdate".
 */
export interface RowUpdate {
  number: number;
  person: string;
  after: number;
  parts: FeedPart[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RowUpdatesPage".
 */
export interface RowUpdatesPage {
  updates: RowUpdate[];
  next: number | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NewEdition".
 */
export interface NewEdition {
  new_edition: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchMessage".
 */
export interface SearchMessage {
  id: string;
  number: number;
  created_at: string;
  version: number;
  conversation: string;
  thread: string | null;
  edited: boolean;
  participants: string[] | null;
  author: string;
  text: string;
  mentions: string[];
  level: Level | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchDeleted".
 */
export interface SearchDeleted {
  id: string;
  version: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchCreated".
 */
export interface SearchCreated {
  kind: "message.created";
  org: string;
  reset: number;
  number: number;
  message: SearchMessage;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchChanged".
 */
export interface SearchChanged {
  kind: "message.changed";
  org: string;
  reset: number;
  number: number;
  message: SearchMessage;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchDeletedChange".
 */
export interface SearchDeletedChange {
  kind: "message.deleted";
  org: string;
  reset: number;
  number: number;
  message: SearchDeleted;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchApplied".
 */
export interface SearchApplied {
  applied: boolean;
  already?: boolean;
  wipe: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchRefusal".
 */
export interface SearchRefusal {
  refused: string;
  wipe: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NoteDue".
 */
export interface NoteDue {
  kind: "due";
  org: string;
  reset: number;
  number: number;
  previous: number;
  edition: number;
  at: string;
  name: string;
  row: RowKey;
  person: string;
  bucket: Bucket;
  why: string;
  text: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NoteChanged".
 */
export interface NoteChanged {
  kind: "changed";
  org: string;
  reset: number;
  number: number;
  previous: number;
  edition: number;
  at: string;
  name: string;
  text: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NoteWithdrawn".
 */
export interface NoteWithdrawn {
  kind: "withdrawn";
  org: string;
  reset: number;
  number: number;
  previous: number;
  edition: number;
  at: string;
  name: string;
  because: WithdrawReason;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NoteClock".
 */
export interface NoteClock {
  kind: "clock";
  org: string;
  reset: number;
  number: number;
  previous: number;
  edition: number;
  at: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NoteTaken".
 */
export interface NoteTaken {
  taken: boolean;
  already?: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NoteBehind".
 */
export interface NoteBehind {
  i_am_at: number;
  edition: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NotificationEntry".
 */
export interface NotificationEntry {
  name: string;
  person: string;
  state: NoteState;
  at: string | null;
  delayed: boolean;
  why: string;
  text: string;
  now: NoteNow | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentEventMentioned".
 */
export interface AgentEventMentioned {
  kind: "mentioned";
  org: string;
  number: number;
  reset: number;
  agent_chain: number;
  message: Message;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentEventDirectMessage".
 */
export interface AgentEventDirectMessage {
  kind: "direct_message";
  org: string;
  number: number;
  reset: number;
  agent_chain: number;
  message: Message;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentEventReply".
 */
export interface AgentEventReply {
  kind: "reply";
  org: string;
  number: number;
  reset: number;
  agent_chain: number;
  message: Message;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentEventChanged".
 */
export interface AgentEventChanged {
  kind: "changed";
  org: string;
  number: number;
  reset: number;
  agent_chain: number;
  message: Message;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentEventDeleted".
 */
export interface AgentEventDeleted {
  kind: "deleted";
  org: string;
  number: number;
  reset: number;
  agent_chain: number;
  message: Message;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentEventReaction".
 */
export interface AgentEventReaction {
  kind: "reaction";
  org: string;
  number: number;
  reset: number;
  agent_chain: number;
  message: Message;
  by: string;
  emoji: string;
  added: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentEvents".
 */
export interface AgentEvents {
  reset: number;
  continue_from: number | null;
  events: AgentEvent[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "TooFarBehind".
 */
export interface TooFarBehind {
  too_far_behind: boolean;
  oldest: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentPost".
 */
export interface AgentPost {
  id: string;
  conversation: string;
  thread: string | null;
  text: string;
  level: Level | null;
  because_of: number | null;
  mentions?: string[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentPosted".
 */
export interface AgentPosted {
  message: string;
  number: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentReplyStart".
 */
export interface AgentReplyStart {
  id: string;
  conversation: string;
  thread: string | null;
  because_of: number | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentPiece".
 */
export interface AgentPiece {
  at: number;
  text: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentReplyFinish".
 */
export interface AgentReplyFinish {
  id: string;
  level: Level | null;
  mentions: string[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PieceMissing".
 */
export interface PieceMissing {
  have: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentStatus".
 */
export interface AgentStatus {
  state: AgentState;
  on: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentReaction".
 */
export interface AgentReaction {
  id: string;
  message: string;
  emoji: string;
  on: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentDirect".
 */
export interface AgentDirect {
  with: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "AgentDirectAnswer".
 */
export interface AgentDirectAnswer {
  conversation: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "FeedPlace".
 */
export interface FeedPlace {
  reset: number;
  through: number;
  edition: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SearchPlace".
 */
export interface SearchPlace {
  reset: number;
  through: number;
  wipe: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SenderPlace".
 */
export interface SenderPlace {
  reset: number;
  through: number;
  edition: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ReceiverStatus".
 */
export interface ReceiverStatus {
  name: ReceiverName;
  place: number;
  waiting: number;
  state: ReceiverState;
  parked: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "DeliveryStatus".
 */
export interface DeliveryStatus {
  receivers: ReceiverStatus[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ParkedChange".
 */
export interface ParkedChange {
  receiver: ReceiverName;
  number: number;
  reason: string;
  tries: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ParkedList".
 */
export interface ParkedList {
  parked: ParkedChange[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ModeBody".
 */
export interface ModeBody {
  mode: Mode;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RebuildRequest".
 */
export interface RebuildRequest {
  receiver: ReceiverName;
  rules_changed: boolean;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "RestartRequest".
 */
export interface RestartRequest {
  edition: number | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "NotificationList".
 */
export interface NotificationList {
  notifications: NotificationEntry[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "PeopleList".
 */
export interface PeopleList {
  people: PersonView[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ScriptStep".
 */
export interface ScriptStep {
  at: string;
  who: string;
  what: string;
  situation: string | null;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Situation".
 */
export interface Situation {
  id: string;
  title: string;
  outcome: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ScriptState".
 */
export interface ScriptState {
  running: boolean;
  pace: number;
  happened: ScriptStep[];
  next: ScriptStep[];
  situations: Situation[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "ScriptRequest".
 */
export interface ScriptRequest {
  do: ScriptCommand;
  pace?: number;
  situation?: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SeedOpened".
 */
export interface SeedOpened {
  session: string;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SeedBatch".
 */
export interface SeedBatch {
  changes: Change[];
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "SeedThrough".
 */
export interface SeedThrough {
  through: number;
}
/**
 * This interface was referenced by `ContractTypes`'s JSON-Schema
 * via the `definition` "Empty".
 */
export interface Empty {}
