import { useVideoPlayer, VideoView } from "expo-video";
import { useEffect, useMemo } from "react";

import { getMediaHeaders } from "@/lib/media";
import { useTheme } from "@/theme/theme-provider";

export type VideoPlayerProps = { uri: string; isVisible?: boolean };

export function VideoPlayer({ uri, isVisible = true }: VideoPlayerProps) {
	const theme = useTheme();
	// Media endpoints require a Bearer token — memoize on uri so the player
	// isn't rebuilt by unrelated re-renders (which would restart playback).
	const source = useMemo(() => {
		const headers = getMediaHeaders();
		return Object.keys(headers).length > 0 ? { uri, headers } : uri;
	}, [uri]);
	const player = useVideoPlayer(source);

	// Stop playback once the card scrolls out of view — otherwise it keeps
	// playing (and downloading) off-screen indefinitely.
	useEffect(() => {
		if (!isVisible) player.pause();
	}, [isVisible, player]);

	return (
		<VideoView
			player={player}
			style={{
				width: "100%",
				height: 220,
				backgroundColor: theme.colors.divider,
				borderRadius: 16,
				marginVertical: 8,
			}}
			nativeControls
			contentFit="cover"
		/>
	);
}
