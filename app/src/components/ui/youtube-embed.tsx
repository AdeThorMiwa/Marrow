import { View } from 'react-native';
import { WebView } from 'react-native-webview';

import { useTheme } from '@/theme/theme-provider';

export type YouTubeEmbedProps = { videoId: string; isVisible?: boolean };

// YouTube video blocks carry a video ID, not a raw playable file URL (see
// getYoutubeVideoId's doc comment) — so unlike VideoPlayer/AudioPlayer,
// which hand a direct URL to a native player, this has to embed YouTube's
// own iframe player. There's no player instance to call .pause() on the
// way VideoPlayer/AudioPlayer do for their isVisible effect, so the same
// "stop playback once scrolled off-screen" behavior is achieved by simply
// not mounting the WebView at all while off-screen.
export function YouTubeEmbed({ videoId, isVisible = true }: YouTubeEmbedProps) {
  const theme = useTheme();

  return (
    <View
      style={{
        width: '100%',
        height: 220,
        borderRadius: 16,
        overflow: 'hidden', // WebView itself doesn't respect borderRadius on Android
        marginVertical: 8,
        backgroundColor: theme.colors.divider,
      }}>
      {isVisible ? (
        <WebView
          source={{ uri: `https://www.youtube.com/embed/${videoId}` }}
          style={{ flex: 1, backgroundColor: theme.colors.divider }}
          allowsFullscreenVideo
          mediaPlaybackRequiresUserAction
        />
      ) : null}
    </View>
  );
}
