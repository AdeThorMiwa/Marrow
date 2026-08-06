import { Image } from 'expo-image';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, RefreshControl, useWindowDimensions, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { AudioPlayer, Badge, Button, Markdown, SourceLogo, Text, VideoPlayer, YouTubeEmbed } from '@/components/ui';
import { ApiError } from '@/lib/api';
import { getFeed } from '@/lib/feed';
import { getPlayableUrl, getYoutubeVideoId } from '@/lib/media';
import type { ContentPayload, FeedItem, SourceHealthPayload } from '@/lib/types';
import { useTheme } from '@/theme/theme-provider';

// Twitter-style single-column timeline: on a wide viewport, half the
// viewport, centered, capped at the design system's readable measure
// (maxContentWidth), with a border around the whole column. Below the
// breakpoint (any narrow viewport, not just native — a resized browser
// window collapses the same way), the column goes full width, edge to edge,
// no side border.
const DESKTOP_BREAKPOINT = 768;

// How often to check for new content at the top of the feed — new items
// never auto-insert (that would yank the scroll position out from under
// whoever's mid-read); they queue behind the "new posts" pill instead,
// same as Twitter/X.
const NEW_CONTENT_POLL_MS = 30_000;

function feedItemKeyOf(item: FeedItem): string {
  if (item.type === 'source_health') return `source_health:${item.payload.source_id}`;
  return `content:${item.payload.content_id}`;
}

export default function HomeScreen() {
  const theme = useTheme();
  const { width } = useWindowDimensions();
  const isDesktop = width >= DESKTOP_BREAKPOINT;
  const horizontalInset = isDesktop ? theme.spacing.lg : theme.spacing.md;
  const columnBorderWidth = isDesktop ? theme.hairlineWidth : 0;

  const [items, setItems] = useState<FeedItem[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [loadedOnce, setLoadedOnce] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [newItems, setNewItems] = useState<FeedItem[]>([]);

  const flatListRef = useRef<FlatList<FeedItem>>(null);
  // Mirrors `items`/`newItems` for the polling effect below — a plain
  // interval closes over stale state otherwise, and re-registering the
  // interval on every items change would reset its cadence.
  const knownKeysRef = useRef<Set<string>>(new Set());

  // Which cards are actually on screen right now — video/audio players read
  // this to pause themselves once their card scrolls out of view. FlatList
  // forbids `onViewableItemsChanged`/`viewabilityConfig` changing identity
  // across renders, so both are created once via a lazy initializer rather
  // than a plain ref read (the latter trips the "no ref access during
  // render" rule, even for the common ref-as-stable-identity pattern).
  const [visibleKeys, setVisibleKeys] = useState<Set<string>>(new Set());
  const [onViewableItemsChanged] = useState(
    () =>
      ({ viewableItems }: { viewableItems: { key: string }[] }) => {
        setVisibleKeys(new Set(viewableItems.map((v) => v.key)));
      }
  );
  const [viewabilityConfig] = useState(() => ({ itemVisiblePercentThreshold: 50 }));

  const loadFirstPage = useCallback(async () => {
    setError(null);
    try {
      const page = await getFeed();
      setItems(page.items);
      setNewItems([]);
      knownKeysRef.current = new Set(page.items.map(feedItemKeyOf));
      setCursor(page.next_cursor || undefined);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to load feed.');
    } finally {
      setLoadedOnce(true);
    }
  }, []);

  useEffect(() => {
    let ignore = false;
    (async () => {
      setError(null);
      try {
        const page = await getFeed();
        if (ignore) return;
        setItems(page.items);
        knownKeysRef.current = new Set(page.items.map(feedItemKeyOf));
        setCursor(page.next_cursor || undefined);
      } catch (e) {
        if (!ignore) setError(e instanceof ApiError ? e.message : 'Failed to load feed.');
      } finally {
        if (!ignore) setLoadedOnce(true);
      }
    })();
    return () => {
      ignore = true;
    };
  }, []);

  // Mirrors items.length for the polling effect — lets it tell "the feed is
  // genuinely empty/failed to load" apart from "there's a page showing,
  // queue new items behind the pill instead" without a stale closure.
  const itemsLengthRef = useRef(0);
  useEffect(() => {
    itemsLengthRef.current = items.length;
  }, [items]);

  // Twitter-style "new posts" pill — poll the first page and stash anything
  // not already known, rather than inserting straight into `items` (which
  // would yank the scroll position out from under an in-progress read). If
  // there's nothing on screen yet (e.g. the initial load failed), a
  // successful poll populates directly instead — there's no scroll position
  // to protect, and the error banner shouldn't linger once the feed is
  // reachable again.
  useEffect(() => {
    if (!loadedOnce) return;

    const interval = setInterval(async () => {
      try {
        const page = await getFeed();
        setError(null);

        const fresh = page.items.filter((item) => !knownKeysRef.current.has(feedItemKeyOf(item)));
        if (fresh.length === 0) return;

        if (itemsLengthRef.current === 0) {
          setItems(fresh);
          fresh.forEach((item) => knownKeysRef.current.add(feedItemKeyOf(item)));
          setCursor(page.next_cursor || undefined);
          return;
        }

        setNewItems((prev) => {
          const seen = new Set(prev.map(feedItemKeyOf));
          const merged = [...fresh.filter((item) => !seen.has(feedItemKeyOf(item))), ...prev];
          merged.forEach((item) => knownKeysRef.current.add(feedItemKeyOf(item)));
          return merged;
        });
      } catch {
        // A missed poll isn't worth surfacing — the next interval tries again.
      }
    }, NEW_CONTENT_POLL_MS);

    return () => clearInterval(interval);
  }, [loadedOnce]);

  const showNewItems = useCallback(() => {
    setItems((prev) => [...newItems, ...prev]);
    setNewItems([]);
    flatListRef.current?.scrollToOffset({ offset: 0, animated: true });
  }, [newItems]);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await loadFirstPage();
    setRefreshing(false);
  }, [loadFirstPage]);

  const onEndReached = useCallback(async () => {
    if (!cursor || loadingMore) return;
    setLoadingMore(true);
    try {
      const page = await getFeed(cursor);
      setItems((prev) => [...prev, ...page.items]);
      page.items.forEach((item) => knownKeysRef.current.add(feedItemKeyOf(item)));
      setCursor(page.next_cursor || undefined);
    } catch {
      // Pagination failure isn't fatal — the page already loaded stays visible;
      // the next pull-to-refresh or scroll retry will try again.
    } finally {
      setLoadingMore(false);
    }
  }, [cursor, loadingMore]);

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.colors.background }}>
      <View
        style={
          isDesktop
            ? { flex: 1, width: '50%', maxWidth: theme.maxContentWidth, alignSelf: 'center' }
            : { flex: 1 }
        }>
        <View
          style={{
            flex: 1,
            borderLeftWidth: columnBorderWidth,
            borderRightWidth: columnBorderWidth,
            borderColor: theme.colors.divider,
          }}>
          <View style={{ paddingHorizontal: horizontalInset, paddingVertical: theme.spacing.md }}>
            <Text variant="itemTitle">Marrow</Text>
          </View>
          <View style={{ height: theme.hairlineWidth, backgroundColor: theme.colors.divider }} />

          {error ? (
            <View style={{ paddingHorizontal: horizontalInset, paddingVertical: theme.spacing.lg, gap: theme.spacing.sm }}>
              <Text variant="body" tone="secondary">
                {error}
              </Text>
              <Button variant="outline" size="sm" onPress={loadFirstPage}>
                Retry
              </Button>
            </View>
          ) : null}

          {loadedOnce && !error && items.length === 0 ? (
            <View style={{ paddingHorizontal: horizontalInset, paddingVertical: theme.spacing.lg }}>
              <Text variant="body" tone="secondary">
                Nothing here yet. Add a source to start pulling in content.
              </Text>
            </View>
          ) : null}

          <View style={{ flex: 1, position: 'relative' }}>
            {newItems.length > 0 ? (
              <View
                style={{
                  position: 'absolute',
                  top: theme.spacing.sm,
                  left: 0,
                  right: 0,
                  alignItems: 'center',
                  zIndex: 10,
                }}>
                <Pressable
                  onPress={showNewItems}
                  style={{
                    backgroundColor: theme.colors.ink,
                    borderRadius: theme.radius,
                    paddingHorizontal: theme.spacing.md,
                    paddingVertical: theme.spacing.sm,
                  }}>
                  <Text variant="label" style={{ color: theme.colors.background }}>
                    ↑ {newItems.length} new {newItems.length === 1 ? 'post' : 'posts'}
                  </Text>
                </Pressable>
              </View>
            ) : null}

            <FlatList
              ref={flatListRef}
              data={items}
              keyExtractor={feedItemKeyOf}
              refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} />}
              onEndReachedThreshold={0.5}
              onEndReached={onEndReached}
              onViewableItemsChanged={onViewableItemsChanged}
              viewabilityConfig={viewabilityConfig}
              ItemSeparatorComponent={() => (
                <View style={{ height: theme.hairlineWidth, backgroundColor: theme.colors.divider }} />
              )}
              ListFooterComponent={
                loadingMore ? (
                  <View style={{ paddingVertical: theme.spacing.lg }}>
                    <ActivityIndicator color={theme.colors.ink} />
                  </View>
                ) : null
              }
              renderItem={({ item }) => (
                <FeedRow item={item} horizontalInset={horizontalInset} isVisible={visibleKeys.has(feedItemKeyOf(item))} />
              )}
            />
          </View>
        </View>
      </View>
    </SafeAreaView>
  );
}

function FeedRow({
  item,
  horizontalInset,
  isVisible,
}: {
  item: FeedItem;
  horizontalInset: number;
  isVisible: boolean;
}) {
  if (item.type === 'source_health') return <SourceHealthRow payload={item.payload} horizontalInset={horizontalInset} />;
  return <ContentRow type={item.type} payload={item.payload} horizontalInset={horizontalInset} isVisible={isVisible} />;
}

// "1d" if published on an earlier calendar day than today, null if posted
// today — matches the "@source • 1d" header, with the "• Nd" part omitted
// entirely for same-day posts.
function daysAgoLabel(publishedAt: string): string | null {
  const published = new Date(publishedAt);
  const now = new Date();
  if (published.toDateString() === now.toDateString()) return null;

  const days = Math.max(1, Math.floor((now.getTime() - published.getTime()) / (1000 * 60 * 60 * 24)));
  if (days < 7) return `${days}d`;
  if (days < 30) return `${Math.floor(days / 7)}w`;
  return `${Math.floor(days / 30)}mo`;
}

function ContentRow({
  type,
  payload,
  horizontalInset,
  isVisible,
}: {
  type: 'text' | 'video' | 'audio';
  payload: ContentPayload;
  horizontalInset: number;
  isVisible: boolean;
}) {
  const theme = useTheme();
  const daysAgo = daysAgoLabel(payload.published_at);

  return (
    <View style={{ paddingHorizontal: horizontalInset, paddingVertical: theme.spacing.md, gap: theme.spacing.xs }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: theme.spacing.xs }}>
        <SourceLogo adapterId={payload.source_adapter_id} size={20} />
        <Text variant="caption" tone="secondary">
          @{payload.source_name}
          {daysAgo ? ` • ${daysAgo}` : ''}
        </Text>
      </View>
      {payload.title ? <Text variant="itemTitle">{payload.title}</Text> : null}
      <ContentMedia type={type} blocks={payload.blocks} isVisible={isVisible} />
      {payload.summary ? <Markdown size="small">{payload.summary}</Markdown> : null}
    </View>
  );
}

// The one media slot per card — video wins over audio wins over the first
// image, matching FeedItem.Type's own priority (Type is computed the same
// way on the backend, so in practice exactly one of these branches ever
// finds a block).
function ContentMedia({
  type,
  blocks,
  isVisible,
}: {
  type: 'text' | 'video' | 'audio';
  blocks: ContentPayload['blocks'];
  isVisible: boolean;
}) {
  if (type === 'video') {
    const block = blocks.find((b) => b.kind === 'video');
    const youtubeVideoId = block ? getYoutubeVideoId(block) : undefined;
    if (youtubeVideoId) return <YouTubeEmbed videoId={youtubeVideoId} isVisible={isVisible} />;

    const uri = block ? getPlayableUrl(block) : undefined;
    if (uri) return <VideoPlayer uri={uri} isVisible={isVisible} />;
  }

  if (type === 'audio') {
    const block = blocks.find((b) => b.kind === 'audio');
    const uri = block ? getPlayableUrl(block) : undefined;
    if (uri) return <AudioPlayer uri={uri} isVisible={isVisible} />;
  }

  const imageBlock = blocks.find((b) => b.kind === 'image');
  const imageUri = imageBlock ? getPlayableUrl(imageBlock) : undefined;
  if (imageUri) {
    return <Image source={{ uri: imageUri }} style={{ width: '100%', height: 220, borderRadius: 16, marginVertical: 8 }} contentFit="cover" />;
  }

  return null;
}

function SourceHealthRow({ payload, horizontalInset }: { payload: SourceHealthPayload; horizontalInset: number }) {
  const theme = useTheme();
  return (
    <View style={{ paddingHorizontal: horizontalInset, paddingVertical: theme.spacing.md, gap: theme.spacing.xs }}>
      <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Text variant="body">{payload.source_name}</Text>
        <Badge variant="solid">{payload.health_status}</Badge>
      </View>
      <Text variant="caption" tone="secondary">
        Hasn&apos;t updated recently — check this source.
      </Text>
    </View>
  );
}
