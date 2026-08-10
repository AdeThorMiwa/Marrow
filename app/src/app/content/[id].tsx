import { Image } from 'expo-image';
import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { AudioPlayer, Button, CommentThread, Markdown, Text, VideoPlayer, YouTubeEmbed } from '@/components/ui';
import { ApiError } from '@/lib/api';
import { getComments, getContentDetail } from '@/lib/content';
import { getPlayableUrl, getYoutubeVideoId } from '@/lib/media';
import type { BlockDetail, Comment, ContentDetail } from '@/lib/types';
import { useTheme } from '@/theme/theme-provider';

export default function ContentDetailScreen() {
  const theme = useTheme();
  const { id } = useLocalSearchParams<{ id: string }>();

  const [detail, setDetail] = useState<ContentDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const [comments, setComments] = useState<Comment[] | null>(null);
  const [commentsLoading, setCommentsLoading] = useState(false);
  const [commentsError, setCommentsError] = useState<string | null>(null);

  useEffect(() => {
    let ignore = false;
    setLoading(true);
    setError(null);
    getContentDetail(id)
      .then((data) => {
        if (!ignore) setDetail(data);
      })
      .catch((e) => {
        if (!ignore) setError(e instanceof ApiError ? e.message : 'Failed to load content.');
      })
      .finally(() => {
        if (!ignore) setLoading(false);
      });
    return () => {
      ignore = true;
    };
  }, [id]);

  // Comments are never fetched automatically — only on explicit tap
  // (Content Detail Requirement 2.3).
  const loadComments = useCallback(async () => {
    setCommentsLoading(true);
    setCommentsError(null);
    try {
      const thread = await getComments(id);
      setComments(thread.comments);
    } catch (e) {
      setCommentsError(e instanceof ApiError ? e.message : 'Failed to load comments.');
    } finally {
      setCommentsLoading(false);
    }
  }, [id]);

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.colors.background }}>
      <View
        style={{
          flexDirection: 'row',
          alignItems: 'center',
          paddingHorizontal: theme.spacing.lg,
          paddingVertical: theme.spacing.md,
        }}>
        <Button variant="ghost" size="sm" onPress={() => router.back()}>
          Back
        </Button>
      </View>

      {loading ? (
        <View style={{ flex: 1, alignItems: 'center', justifyContent: 'center' }}>
          <ActivityIndicator color={theme.colors.ink} />
        </View>
      ) : null}

      {error ? (
        <View style={{ paddingHorizontal: theme.spacing.lg, gap: theme.spacing.sm }}>
          <Text variant="body" tone="secondary">
            {error}
          </Text>
        </View>
      ) : null}

      {detail ? (
        <ScrollView contentContainerStyle={{ padding: theme.spacing.lg, gap: theme.spacing.md }}>
          <Text variant="caption" tone="secondary">
            @{detail.source_name}
          </Text>
          {detail.title ? <Text variant="heading2">{detail.title}</Text> : null}

          {detail.blocks.map((block, i) => (
            <ContentDetailBlock key={i} block={block} />
          ))}
          {/* Content.Description — separate from Blocks, e.g. a video's synopsis */}
          {detail.description ? <Markdown>{detail.description}</Markdown> : null}

          <View style={{ height: theme.hairlineWidth, backgroundColor: theme.colors.divider, marginVertical: theme.spacing.sm }} />

          {detail.has_comments ? (
            <CommentsSection
              comments={comments}
              loading={commentsLoading}
              error={commentsError}
              onLoad={loadComments}
            />
          ) : null}
        </ScrollView>
      ) : null}
    </SafeAreaView>
  );
}

function ContentDetailBlock({ block }: { block: BlockDetail }) {
  if (block.kind === 'text') {
    return block.markdown ? <Markdown>{block.markdown}</Markdown> : null;
  }

  if (block.kind === 'video') {
    const youtubeVideoId = getYoutubeVideoId(block);
    if (youtubeVideoId) return <YouTubeEmbed videoId={youtubeVideoId} isVisible />;
    const uri = getPlayableUrl(block);
    if (uri) return <VideoPlayer uri={uri} isVisible />;
    return null;
  }

  if (block.kind === 'audio') {
    const uri = getPlayableUrl(block);
    return uri ? <AudioPlayer uri={uri} isVisible /> : null;
  }

  const uri = getPlayableUrl(block);
  return uri ? (
    <Image source={{ uri }} style={{ width: '100%', height: 260, borderRadius: 16 }} contentFit="cover" />
  ) : null;
}

function CommentsSection({
  comments,
  loading,
  error,
  onLoad,
}: {
  comments: Comment[] | null;
  loading: boolean;
  error: string | null;
  onLoad: () => void;
}) {
  const theme = useTheme();

  if (comments === null) {
    return (
      <View style={{ gap: theme.spacing.sm }}>
        <Button variant="outline" onPress={onLoad} disabled={loading}>
          {loading ? 'Loading comments…' : 'Load comments'}
        </Button>
        {error ? (
          <Text variant="caption" tone="secondary">
            {error}
          </Text>
        ) : null}
      </View>
    );
  }

  return (
    <View style={{ gap: theme.spacing.md }}>
      <Text variant="label">Comments</Text>
      <CommentThread comments={comments} />
    </View>
  );
}
