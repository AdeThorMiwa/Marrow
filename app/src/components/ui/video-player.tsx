import { useVideoPlayer, VideoView } from 'expo-video';
import { useEffect } from 'react';

import { useTheme } from '@/theme/theme-provider';

export type VideoPlayerProps = { uri: string; isVisible?: boolean };

export function VideoPlayer({ uri, isVisible = true }: VideoPlayerProps) {
  const theme = useTheme();
  const player = useVideoPlayer(uri);

  // Stop playback once the card scrolls out of view — otherwise it keeps
  // playing (and downloading) off-screen indefinitely.
  useEffect(() => {
    if (!isVisible) player.pause();
  }, [isVisible, player]);

  return (
    <VideoView
      player={player}
      style={{ width: '100%', height: 220, backgroundColor: theme.colors.divider, borderRadius: 16, marginVertical: 8 }}
      nativeControls
      contentFit="cover"
    />
  );
}
