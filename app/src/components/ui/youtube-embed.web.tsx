import { useTheme } from '@/theme/theme-provider';

export type YouTubeEmbedProps = { videoId: string; isVisible?: boolean };

// Native (youtube-embed.tsx) embeds via react-native-webview, which has no
// web target — this is the web counterpart, a plain <iframe> using
// YouTube's own embed endpoint. Same isVisible behavior: don't mount the
// iframe at all while off-screen, since there's no player instance to
// pause directly.
export function YouTubeEmbed({ videoId, isVisible = true }: YouTubeEmbedProps) {
  const theme = useTheme();
  const style = {
    width: '100%',
    height: 220,
    borderRadius: 16,
    marginVertical: 8,
    backgroundColor: theme.colors.divider,
    border: 'none',
  } as const;

  if (!isVisible) return <div style={style} />;

  return (
    <iframe
      src={`https://www.youtube.com/embed/${videoId}`}
      style={style}
      allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
      allowFullScreen
    />
  );
}
