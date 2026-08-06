export type SourceHealth = 'ok' | 'stale' | 'broken';

export type SourceConfig = {
  identifier: string;
  adapter_id: string;
  name: string;
};

export type Source = {
  id: string;
  adapter_id: string;
  identifier: string;
  name: string;
  health: SourceHealth;
  consecutive_failures: number;
  last_fetched_at?: string;
  next_poll_at: string;
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
};

export type SourceHealthPayload = {
  source_id: string;
  source_name: string;
  health_status: SourceHealth;
  last_fetched_at?: string;
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
