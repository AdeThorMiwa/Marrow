import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Image } from 'expo-image';
import { router } from 'expo-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, RefreshControl, useWindowDimensions, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ActionSheet, AudioPlayer, Badge, Button, ConfirmDialog, CreateGroupDialog, Markdown, SourceLogo, Text, VideoPlayer, YouTubeEmbed } from '@/components/ui';
import { ApiError } from '@/lib/api';
import { getFeed, type FeedFilter } from '@/lib/feed';
import { addSourceToGroup, createGroup, listGroups, pauseGroup, unpauseGroup } from '@/lib/group';
import { getPlayableUrl, getYoutubeVideoId } from '@/lib/media';
import { deleteSource, listSources, pauseSource, unpauseSource } from '@/lib/source';
import type { ContentPayload, FeedItem, Group, Source, SourceHealthPayload } from '@/lib/types';
import { useTheme } from '@/theme/theme-provider';

// Fixed width for each item in the source rail — long names ellipsize
// rather than growing the container (Pressable) or shifting neighbors.
const SOURCE_RAIL_ITEM_WIDTH = 64;
const SOURCE_RAIL_LOGO_SIZE = 56;

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

// Matches model.DefaultGroupID (api/internal/model/group.go) — every
// source always belongs to this group, so selecting it means "everything."
const DEFAULT_GROUP_ID = 'default';

// See docs/feed-filtering/design.md §7.
type FilterSelection = { sourceIds: Set<string>; groupIds: Set<string> };

const DEFAULT_SELECTION: FilterSelection = { sourceIds: new Set(), groupIds: new Set([DEFAULT_GROUP_ID]) };

function toggleFilterItem(prev: FilterSelection, kind: 'source' | 'group', id: string): FilterSelection {
  if (kind === 'group' && id === DEFAULT_GROUP_ID) {
    return { sourceIds: new Set(), groupIds: new Set([DEFAULT_GROUP_ID]) }; // Requirement 2.3
  }

  const sourceIds = new Set(prev.sourceIds);
  const groupIds = new Set(prev.groupIds);
  groupIds.delete(DEFAULT_GROUP_ID); // Requirement 2.2

  const target = kind === 'source' ? sourceIds : groupIds;
  if (target.has(id)) target.delete(id);
  else target.add(id);

  if (sourceIds.size === 0 && groupIds.size === 0) {
    return DEFAULT_SELECTION; // Requirement 2.4
  }
  return { sourceIds, groupIds };
}

function toFeedFilter(selection: FilterSelection): FeedFilter {
  return { sourceIds: [...selection.sourceIds], groupIds: [...selection.groupIds] };
}

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

  const [sources, setSources] = useState<Source[]>([]);
  const loadSources = useCallback(async () => {
    try {
      setSources(await listSources());
    } catch {
      // The rail just stays empty (plus the "add source" button) — not
      // worth a whole-screen error for a secondary element.
    }
  }, []);
  useEffect(() => {
    loadSources();
  }, [loadSources]);

  // Bumped by pull-to-refresh so the source rail (sources + groups) refetches
  // alongside the feed — otherwise a rail that failed to load on mount (e.g.
  // a transient backend error) stays blank forever with no retry trigger.
  const [railRefreshSignal, setRailRefreshSignal] = useState(0);

  const [items, setItems] = useState<FeedItem[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [loadedOnce, setLoadedOnce] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [newItems, setNewItems] = useState<FeedItem[]>([]);

  // Active source/group filter — see docs/feed-filtering/design.md §7.
  const [selection, setSelection] = useState<FilterSelection>(DEFAULT_SELECTION);
  // Mirrors `selection` for the polling interval/onEndReached closures below
  // — same stale-closure reason knownKeysRef exists.
  const selectionRef = useRef<FilterSelection>(selection);
  useEffect(() => {
    selectionRef.current = selection;
  }, [selection]);

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
      const page = await getFeed(undefined, undefined, toFeedFilter(selectionRef.current));
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

  // Mount, and again whenever the active filter changes (Requirement 2 —
  // reloads exactly the way it already does on mount, no separate reset
  // logic needed).
  useEffect(() => {
    let ignore = false;
    (async () => {
      setError(null);
      try {
        const page = await getFeed(undefined, undefined, toFeedFilter(selection));
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
  }, [selection]);

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
        const page = await getFeed(undefined, undefined, toFeedFilter(selectionRef.current));
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
    flatListRef.current?.scrollToOffset({ offset: 0, animated: false });
  }, [newItems]);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    setRailRefreshSignal((prev) => prev + 1);
    await Promise.all([loadFirstPage(), loadSources()]);
    setRefreshing(false);
  }, [loadFirstPage, loadSources]);

  const onEndReached = useCallback(async () => {
    if (!cursor || loadingMore) return;
    setLoadingMore(true);
    try {
      const page = await getFeed(cursor, undefined, toFeedFilter(selectionRef.current));
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

          <SourceRail
            sources={sources}
            horizontalInset={horizontalInset}
            onDeleted={(id) => setSources((prev) => prev.filter((s) => s.id !== id))}
            onSourcePausedChanged={(id, paused) =>
              setSources((prev) => prev.map((s) => (s.id === id ? { ...s, paused } : s)))
            }
            selection={selection}
            onToggle={(kind, id) => setSelection((prev) => toggleFilterItem(prev, kind, id))}
            refreshSignal={railRefreshSignal}
          />
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
              // Cards can mount a real native video/audio player, not just
              // text — expensive enough that FlatList's default
              // virtualization window (renders ~21 screens worth of content
              // around the viewport) is what's actually slow here, not
              // wasted re-renders (React Compiler already memoizes those at
              // build time — see app.json's experiments.reactCompiler).
              // Rendering a much narrower window fixes it at the source.
              windowSize={5}
              showsVerticalScrollIndicator={false}
              initialNumToRender={6}
              maxToRenderPerBatch={4}
              removeClippedSubviews
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

  const content = (
    <View style={{ paddingHorizontal: horizontalInset, paddingVertical: theme.spacing.md, gap: theme.spacing.xs }}>
      <View style={{ flexDirection: 'row', justifyContent: 'space-between',  gap: theme.spacing.xs }}>
        <Text variant="caption" tone="secondary">
          @{payload.source_name}
          {daysAgo ? ` • ${daysAgo}` : ''}
        </Text>
        <SourceLogo adapterId={payload.source_adapter_id} size={20} />
      </View>
      {payload.title ? <Text variant="itemTitle">{payload.title}</Text> : null}
      <ContentMedia type={type} blocks={payload.blocks} isVisible={isVisible} />
      {payload.summary ? <Markdown size="small">{payload.summary}</Markdown> : null}
    </View>
  );

  // Server-computed (Content Detail Requirement 3.3) — a card with nothing
  // further to reveal gets no press affordance at all, not just a disabled
  // one.
  if (!payload.detailable) return content;

  return <Pressable onPress={() => router.push(`/content/${payload.content_id}`)}>{content}</Pressable>;
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
    if (youtubeVideoId)
      return (
        // No-op Pressable claims the responder for taps on the video so
        // they don't bubble to the card's outer Pressable (which would
        // navigate to Content Detail instead of letting YouTube's own
        // play/pause interaction happen).
        <Pressable onPress={() => {}}>
          <YouTubeEmbed videoId={youtubeVideoId} isVisible={isVisible} />
        </Pressable>
      );

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
        {payload.reason ?? "Hasn't updated recently — check this source."}
      </Text>
    </View>
  );
}

// Horizontal rail of the user's added sources, just below the header — the
// first item is always a "+" that opens the add-source screen; the rest
// are the sources themselves, logo (rounded, per-adapter icon) + name
// below in a fixed-width column so a long name ellipsizes instead of
// resizing its neighbors. Rail order: "+" add button, then groups, then
// sources (docs/source-groups/requirements.md Requirement 5.2). Long-
// pressing a source opens an action sheet (add to group, delete);
// confirming a delete removes it via onDeleted.
function SourceRail({
  sources,
  horizontalInset,
  onDeleted,
  onSourcePausedChanged,
  selection,
  onToggle,
  refreshSignal,
}: {
  sources: Source[];
  horizontalInset: number;
  onDeleted: (id: string) => void;
  onSourcePausedChanged: (id: string, paused: boolean) => void;
  selection: FilterSelection;
  onToggle: (kind: 'source' | 'group', id: string) => void;
  refreshSignal: number;
}) {
  const theme = useTheme();
  const [menuSource, setMenuSource] = useState<Source | null>(null);
  const [confirmSource, setConfirmSource] = useState<Source | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [groups, setGroups] = useState<Group[]>([]);
  const [groupPickerSource, setGroupPickerSource] = useState<Source | null>(null);
  const [creatingGroupFor, setCreatingGroupFor] = useState<Source | null>(null);
  const [creatingGroup, setCreatingGroup] = useState(false);
  const [menuGroup, setMenuGroup] = useState<Group | null>(null);

  useEffect(() => {
    let ignore = false;
    listGroups()
      .then((data) => {
        if (!ignore) setGroups(data);
      })
      .catch(() => {
        // Same tolerance as the sources fetch — a secondary rail element.
      });
    return () => {
      ignore = true;
    };
  }, [refreshSignal]);

  const handleDelete = useCallback(async () => {
    if (!confirmSource) return;
    setDeleting(true);
    try {
      await deleteSource(confirmSource.id);
      onDeleted(confirmSource.id);
      setConfirmSource(null);
    } catch {
      // The dialog just stays open with the button re-enabled — the user
      // can retry, same as every other mutation in this app.
    } finally {
      setDeleting(false);
    }
  }, [confirmSource, onDeleted]);

  const handleTogglePauseSource = useCallback(async () => {
    if (!menuSource) return;
    const { id, paused } = menuSource;
    setMenuSource(null);
    try {
      await (paused ? unpauseSource(id) : pauseSource(id));
      onSourcePausedChanged(id, !paused);
    } catch {
      // Silent-fail, same tolerance as the rest of this rail's mutations.
    }
  }, [menuSource, onSourcePausedChanged]);

  const handleTogglePauseGroup = useCallback(async () => {
    if (!menuGroup) return;
    const { id, paused } = menuGroup;
    setMenuGroup(null);
    try {
      await (paused ? unpauseGroup(id) : pauseGroup(id));
      setGroups((prev) => prev.map((g) => (g.id === id ? { ...g, paused: !paused } : g)));
    } catch {
      // Silent-fail, same tolerance as the rest of this rail's mutations.
    }
  }, [menuGroup]);

  const handleAddToExistingGroup = useCallback(
    async (groupId: string) => {
      if (!groupPickerSource) return;
      const sourceId = groupPickerSource.id;
      setGroupPickerSource(null);
      try {
        await addSourceToGroup(sourceId, groupId);
      } catch {
        // Silent-fail, same tolerance as the rest of this rail's mutations.
      }
    },
    [groupPickerSource]
  );

  const handleCreateGroup = useCallback(
    async (name: string, icon: string) => {
      if (!creatingGroupFor) return;
      setCreatingGroup(true);
      try {
        const group = await createGroup(name, icon);
        setGroups((prev) => [...prev, group]);
        await addSourceToGroup(creatingGroupFor.id, group.id);
        setCreatingGroupFor(null);
      } catch {
        // Dialog stays open with the button re-enabled — same retry pattern
        // as handleDelete.
      } finally {
        setCreatingGroup(false);
      }
    },
    [creatingGroupFor]
  );

  const railData = useMemo(
    () => [
      // The default group is implicit (every source is always in it) —
      // nothing useful to show as its own chip.
      ...groups.filter((g) => !g.is_default).map((g) => ({ kind: 'group' as const, key: `g-${g.id}`, group: g })),
      ...sources.map((s) => ({ kind: 'source' as const, key: `s-${s.id}`, source: s })),
    ],
    [groups, sources]
  );

  return (
    <>
      <FlatList
        horizontal
        showsHorizontalScrollIndicator={false}
        data={railData}
        keyExtractor={(item) => item.key}
        // Web's FlatList defaults to flexGrow: 1 on its outer scroll
        // container — inside this screen's column flex layout that means
        // "grow vertically to fill all remaining space" instead of sizing to
        // its own (short, horizontal) content. Without this override the
        // rail eats the whole rest of the screen.
        style={{ flexGrow: 0 }}
        contentContainerStyle={{ paddingHorizontal: horizontalInset, paddingVertical: theme.spacing.sm, gap: theme.spacing.md }}
        ListHeaderComponent={
          <Pressable
            onPress={() => router.push('/sources/add')}
            style={{ width: SOURCE_RAIL_ITEM_WIDTH, alignItems: 'center', gap: theme.spacing.xs, }}>
            <View
              style={{
                width: SOURCE_RAIL_LOGO_SIZE,
                height: SOURCE_RAIL_LOGO_SIZE,
                borderRadius: SOURCE_RAIL_LOGO_SIZE / 2,
                borderWidth: theme.borderWidth,
                borderColor: theme.colors.ink,
                alignItems: 'center',
                justifyContent: 'center',
                backgroundColor: theme.colors.background,
              }}>
              <MaterialCommunityIcons name="plus" size={SOURCE_RAIL_LOGO_SIZE * 0.5} color={theme.colors.ink} />
            </View>
            <Text variant="caption" tone="secondary" numberOfLines={1} ellipsizeMode="tail">
              Add
            </Text>
          </Pressable>
        }
        renderItem={({ item }) =>
          item.kind === 'group' ? (
            <GroupRailItem
              group={item.group}
              selected={selection.groupIds.has(item.group.id)}
              onPress={() => onToggle('group', item.group.id)}
              onLongPress={() => setMenuGroup(item.group)}
            />
          ) : (
            <SourceRailItem
              source={item.source}
              selected={selection.sourceIds.has(item.source.id)}
              onPress={() => onToggle('source', item.source.id)}
              onLongPress={() => setMenuSource(item.source)}
            />
          )
        }
      />

      <ActionSheet
        visible={menuSource !== null}
        onClose={() => setMenuSource(null)}
        actions={[
          {
            label: 'Add to group',
            onPress: () => setGroupPickerSource(menuSource),
          },
          {
            label: menuSource?.paused ? 'Resume' : 'Pause',
            onPress: handleTogglePauseSource,
          },
          {
            label: 'Delete',
            destructive: true,
            onPress: () => setConfirmSource(menuSource),
          },
        ]}
      />

      <ActionSheet
        visible={groupPickerSource !== null}
        onClose={() => setGroupPickerSource(null)}
        actions={[
          { label: '+ New group', onPress: () => setCreatingGroupFor(groupPickerSource) },
          ...groups.filter((g) => !g.is_default).map((g) => ({ label: g.name, onPress: () => handleAddToExistingGroup(g.id) })),
        ]}
      />

      <ActionSheet
        visible={menuGroup !== null}
        onClose={() => setMenuGroup(null)}
        actions={[
          {
            label: menuGroup?.paused ? 'Resume' : 'Pause',
            onPress: handleTogglePauseGroup,
          },
        ]}
      />

      <CreateGroupDialog
        visible={creatingGroupFor !== null}
        loading={creatingGroup}
        onCreate={handleCreateGroup}
        onCancel={() => setCreatingGroupFor(null)}
      />

      <ConfirmDialog
        visible={confirmSource !== null}
        title={`Delete ${confirmSource?.name ?? 'this source'}?`}
        message="You'll stop getting new content from this source in your feed. Anything already saved to your feed stays right where it is — this only stops future updates."
        confirmLabel="Delete"
        confirmLoading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setConfirmSource(null)}
      />
    </>
  );
}

// No per-group color (docs/source-groups/design.md §2) — same
// background/ink circle every other badge in the app already uses. Selected
// state (feed filter active — docs/feed-filtering/design.md §7) is border
// weight only; paused state (docs/pause-source-group/design.md §6) is
// opacity — neither ever uses color, independent of each other.
function GroupRailItem({
  group,
  selected,
  onPress,
  onLongPress,
}: {
  group: Group;
  selected: boolean;
  onPress: () => void;
  onLongPress: () => void;
}) {
  const theme = useTheme();
  return (
    <Pressable
      onPress={onPress}
      onLongPress={onLongPress}
      style={{ width: SOURCE_RAIL_ITEM_WIDTH, alignItems: 'center', gap: theme.spacing.xs, opacity: group.paused ? 0.4 : 1 }}>
      <View
        style={{
          width: SOURCE_RAIL_LOGO_SIZE,
          height: SOURCE_RAIL_LOGO_SIZE,
          borderRadius: SOURCE_RAIL_LOGO_SIZE / 2,
          borderWidth: selected ? theme.borderWidthError : theme.borderWidth,
          borderColor: theme.colors.ink,
          alignItems: 'center',
          justifyContent: 'center',
          backgroundColor: theme.colors.background,
        }}>
        <MaterialCommunityIcons name={group.icon as any} size={SOURCE_RAIL_LOGO_SIZE * 0.5} color={theme.colors.ink} />
      </View>
      <Text variant="caption" tone="secondary" numberOfLines={1} ellipsizeMode="tail" style={{ width: '100%', textAlign: 'center' }}>
        {group.name}
      </Text>
    </Pressable>
  );
}

function SourceRailItem({
  source,
  selected,
  onPress,
  onLongPress,
}: {
  source: Source;
  selected: boolean;
  onPress: () => void;
  onLongPress: () => void;
}) {
  const theme = useTheme();
  return (
    <Pressable
      onPress={onPress}
      onLongPress={onLongPress}
      style={{ width: SOURCE_RAIL_ITEM_WIDTH, alignItems: 'center', gap: theme.spacing.xs, opacity: source.paused ? 0.4 : 1 }}>
      <View
        style={{
          width: SOURCE_RAIL_LOGO_SIZE,
          height: SOURCE_RAIL_LOGO_SIZE,
          borderRadius: SOURCE_RAIL_LOGO_SIZE / 2,
          borderWidth: selected ? theme.borderWidthError : 0,
          borderColor: theme.colors.ink,
          alignItems: 'center',
          justifyContent: 'center',
        }}>
        <SourceLogo adapterId={source.adapter_id} logoUrl={source.logo_url} name={source.name} size={SOURCE_RAIL_LOGO_SIZE} />
      </View>
      <Text variant="caption" tone="secondary" numberOfLines={1} ellipsizeMode="tail" style={{ width: '100%', textAlign: 'center' }}>
        {source.name}
      </Text>
    </Pressable>
  );
}
