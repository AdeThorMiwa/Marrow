import { View } from 'react-native';

import type { Comment } from '@/lib/types';
import { useTheme } from '@/theme/theme-provider';

import { Avatar } from './avatar';
import { Text } from './text';

export type CommentThreadProps = {
  comments: Comment[];
};

// A comment's "1d" if posted an earlier calendar day than today, else the
// time — same shape as the feed card's daysAgoLabel, just always showing
// something (a comment thread has no "today, no label" case worth hiding).
function commentTimeLabel(publishedAt: string): string {
  const published = new Date(publishedAt);
  const now = new Date();
  if (published.toDateString() === now.toDateString()) {
    return published.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
  }
  const days = Math.max(1, Math.floor((now.getTime() - published.getTime()) / (1000 * 60 * 60 * 24)));
  if (days < 7) return `${days}d`;
  if (days < 30) return `${Math.floor(days / 7)}w`;
  return `${Math.floor(days / 30)}mo`;
}

// The API hands back a flat list with reply_to_id links (Design §3/§9) — no
// tree, no depth cap. Nesting is reconstructed here purely for rendering;
// however deep a reply chain actually goes, it renders that deep.
function buildTree(comments: Comment[]): Map<string, Comment[]> {
  const byParent = new Map<string, Comment[]>();
  for (const comment of comments) {
    const key = comment.reply_to_id ?? '';
    const siblings = byParent.get(key) ?? [];
    siblings.push(comment);
    byParent.set(key, siblings);
  }
  return byParent;
}

export function CommentThread({ comments }: CommentThreadProps) {
  const theme = useTheme();
  const byParent = buildTree(comments);
  const topLevel = byParent.get('') ?? [];

  if (comments.length === 0) {
    return (
      <Text variant="body" tone="secondary">
        No comments yet.
      </Text>
    );
  }

  return (
    <View style={{ gap: theme.spacing.md }}>
      {topLevel.map((comment) => (
        <CommentNode key={comment.id} comment={comment} byParent={byParent} depth={0} />
      ))}
    </View>
  );
}

function CommentNode({ comment, byParent, depth }: { comment: Comment; byParent: Map<string, Comment[]>; depth: number }) {
  const theme = useTheme();
  const replies = byParent.get(comment.id) ?? [];

  return (
    <View
      style={
        depth === 0
          ? { gap: theme.spacing.sm }
          : {
              gap: theme.spacing.sm,
              marginLeft: theme.spacing.md,
              paddingLeft: theme.spacing.md,
              borderLeftWidth: theme.hairlineWidth,
              borderLeftColor: theme.colors.divider,
            }
      }>
      <View style={{ flexDirection: 'row', gap: theme.spacing.sm }}>
        <Avatar name={comment.author_name} imageUrl={comment.author_avatar_url} size="sm" />
        <View style={{ flex: 1, gap: 2 }}>
          <View style={{ flexDirection: 'row', gap: theme.spacing.xs, alignItems: 'baseline' }}>
            <Text variant="label">{comment.author_name}</Text>
            <Text variant="caption" tone="secondary">
              {commentTimeLabel(comment.published_at)}
            </Text>
          </View>
          <Text variant="body">{comment.text}</Text>
        </View>
      </View>

      {replies.length > 0 ? (
        <View style={{ gap: theme.spacing.sm }}>
          {replies.map((reply) => (
            <CommentNode key={reply.id} comment={reply} byParent={byParent} depth={depth + 1} />
          ))}
        </View>
      ) : null}
    </View>
  );
}
