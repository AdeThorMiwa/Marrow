import { useState } from 'react';
import { Linking, Pressable, View } from 'react-native';
import YoutubePlayer, { PLAYER_ERRORS, type PLAYER_STATES } from 'react-native-youtube-iframe';

import { useTheme } from '@/theme/theme-provider';

import { Text } from './text';

export type YouTubeEmbedProps = { videoId: string; isVisible?: boolean };

const HEIGHT = 220;

// A bare WebView pointed at youtube.com/embed/{id} (the previous
// implementation) reliably fails on real Android devices — YouTube's
// embed endpoint checks for a real browser environment and rejects a
// generic Android WebView user agent with "video player configuration
// error" (its error 153), even for videos that embed fine everywhere
// else (confirmed live via YouTube's own oEmbed API — not an
// embedding-restriction issue). react-native-youtube-iframe instead loads
// the real YouTube IFrame Player API JS inside the WebView, which is the
// supported, stable path — the same approach YouTube's own IFrame API
// consumers use, not a raw page load.
export function YouTubeEmbed({ videoId, isVisible = true }: YouTubeEmbedProps) {
  const theme = useTheme();
  const [playing, setPlaying] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (error) {
    return (
      <Pressable
        onPress={() => Linking.openURL(`https://www.youtube.com/watch?v=${videoId}`)}
        style={{
          height: HEIGHT,
          borderRadius: 16,
          marginVertical: 8,
          backgroundColor: theme.colors.divider,
          alignItems: 'center',
          justifyContent: 'center',
          gap: theme.spacing.xs,
        }}>
        <Text variant="label">
          {error === PLAYER_ERRORS.EMBED_NOT_ALLOWED ? "This video can't be embedded" : 'Video failed to load'}
        </Text>
        <Text variant="caption" tone="secondary">
          Watch on YouTube ↗
        </Text>
      </Pressable>
    );
  }

  return (
    <View style={{ height: HEIGHT, borderRadius: 16, overflow: 'hidden', marginVertical: 8, backgroundColor: theme.colors.divider }}>
      <YoutubePlayer
        height={HEIGHT}
        videoId={videoId}
        play={isVisible && playing}
        onChangeState={(state: PLAYER_STATES) => {
          if (state === 'playing') setPlaying(true);
          else if (state === 'paused' || state === 'ended') setPlaying(false);
        }}
        onError={setError}
      />
    </View>
  );
}
