export type SourceHealth = 'ok' | 'stale' | 'broken';

export type SourceConfig = {
  identifier: string;
  adapter_id: string;
  name: string;
  // This specific source's own avatar/profile-picture/publication-logo —
  // distinct from the per-adapter platform icon. Optional; falls back to
  // initials when unset.
  logo_url?: string;
};

export type Source = {
  id: string;
  adapter_id: string;
  identifier: string;
  name: string;
  logo_url?: string;
  health: SourceHealth;
  consecutive_failures: number;
  // Underlying error behind the most recent unreachable poll (e.g. an
  // expired auth cookie) — unset when there's nothing more specific than
  // the health status itself.
  failure_reason?: string;
  last_fetched_at?: string;
  next_poll_at: string;
  created_at: string;
};

export type Group = {
  id: string;
  name: string;
  icon: string;
  is_default: boolean;
  created_at: string;
};

export type BlockKind = 'text' | 'audio' | 'video' | 'image';

export type BlockSummary = {
  kind: BlockKind;
  media_ref?: string;
  caption?: string;
};

export type ContentPayload = {
  content_id: string;
  source_name: string;
  // Which adapter this Content's Source came from — used to pick a static
  // per-platform logo (Twitter icon, Instagram icon, ...), not per-account
  // data.
  source_adapter_id: string;
  title?: string;
  published_at: string;
  blocks: BlockSummary[];
  summary?: string;
  // Server-computed (Content Detail Requirement 3.3) — never inferred
  // client-side. True iff the summary is a truncated excerpt of a longer
  // full text, or the source's adapter supports comments.
  detailable: boolean;
};

export type SourceHealthPayload = {
  source_id: string;
  source_name: string;
  health_status: SourceHealth;
  last_fetched_at?: string;
  reason?: string;
};

// FeedItem.type doubles as the dominant block kind for content items — the
// client picks which block to feature off Type directly instead of
// scanning Blocks itself (first image for 'text', the video block for
// 'video', the audio block for 'audio').
export type FeedItem =
  | { type: 'text' | 'video' | 'audio'; payload: ContentPayload }
  | { type: 'source_health'; payload: SourceHealthPayload };

export type FeedPage = {
  items: FeedItem[];
  next_cursor: string;
};

export type BlockDetail = {
  kind: BlockKind;
  markdown?: string;
  media_ref?: string;
  caption?: string;
};

export type ContentDetail = {
  content_id: string;
  source_name: string;
  source_adapter_id: string;
  title?: string;
  description?: string;
  url: string;
  published_at: string;
  blocks: BlockDetail[];
  has_comments: boolean;
};

export type Comment = {
  id: string;
  reply_to_id?: string;
  author_name: string;
  author_avatar_url?: string;
  text: string;
  published_at: string;
};

export type CommentThread = {
  comments: Comment[];
  next_cursor?: string;
};
