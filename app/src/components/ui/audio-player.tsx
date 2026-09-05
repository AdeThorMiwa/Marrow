import Slider from "@react-native-community/slider";
import { useAudioPlayer, useAudioPlayerStatus } from "expo-audio";
import { useEffect, useMemo, useState } from "react";
import { Pressable, View } from "react-native";

import { getMediaHeaders } from "@/lib/media";
import { useTheme } from "@/theme/theme-provider";

import { Text } from "./text";

export type AudioPlayerProps = { uri: string; isVisible?: boolean };

const ICON_SIZE = 32;

export function AudioPlayer({ uri, isVisible = true }: AudioPlayerProps) {
	const theme = useTheme();
	// Media endpoints require a Bearer token — memoize on uri so the player
	// isn't rebuilt by unrelated re-renders (which would restart playback).
	const source = useMemo(() => {
		const headers = getMediaHeaders();
		return Object.keys(headers).length > 0 ? { uri, headers } : uri;
	}, [uri]);
	const player = useAudioPlayer(source);
	const status = useAudioPlayerStatus(player);
	// While the user is actively dragging, show the drag position rather
	// than the player's own currentTime (which lags a seek in progress).
	const [seeking, setSeeking] = useState<number | null>(null);

	// Stop playback once the card scrolls out of view — otherwise audio keeps
	// playing off-screen indefinitely.
	useEffect(() => {
		if (!isVisible) player.pause();
	}, [isVisible, player]);

	const toggle = () => {
		if (status.playing) player.pause();
		else player.play();
	};

	return (
		<View
			style={{
				flexDirection: "row",
				alignItems: "center",
				gap: theme.spacing.sm,
				borderWidth: theme.hairlineWidth,
				borderColor: theme.colors.ink,
				borderRadius: theme.radius,
				padding: theme.spacing.md,
				marginVertical: theme.spacing.md,
			}}
		>
			<Pressable
				accessibilityRole="button"
				accessibilityLabel={status.playing ? "Pause" : "Play audio"}
				onPress={toggle}
				style={{
					width: ICON_SIZE,
					height: ICON_SIZE,
					borderRadius: ICON_SIZE / 2,
					borderWidth: 1,
					borderColor: "#FFFFFF",
					alignItems: "center",
					justifyContent: "center",
				}}
			>
				<Text
					style={{
						color: "#FFFFFF",
						fontSize: ICON_SIZE / 2,
						marginLeft: status.playing ? 0 : 2,
					}}
				>
					{status.playing ? "❚❚" : "▶"}
				</Text>
			</Pressable>

			<Slider
				style={{ flex: 1 }}
				minimumValue={0}
				maximumValue={status.duration || 0}
				value={seeking ?? status.currentTime}
				onSlidingStart={() => setSeeking(status.currentTime)}
				onValueChange={setSeeking}
				onSlidingComplete={(value) => {
					player.seekTo(value);
					setSeeking(null);
				}}
				minimumTrackTintColor="#FFFFFF"
				maximumTrackTintColor="#FFFFFF"
				thumbTintColor="#FFFFFF"
			/>

			{status.duration ? (
				<Text variant="caption" tone="secondary">
					{formatTime(seeking ?? status.currentTime)} /{" "}
					{formatTime(status.duration)}
				</Text>
			) : null}
		</View>
	);
}

function formatTime(seconds: number): string {
	const total = Math.floor(seconds);
	const m = Math.floor(total / 60);
	const s = total % 60;
	return `${m}:${s.toString().padStart(2, "0")}`;
}
