import "@/global.css";

import {
	SmoochSans_400Regular,
	SmoochSans_500Medium,
	SmoochSans_600SemiBold,
	SmoochSans_700Bold,
	useFonts,
} from "@expo-google-fonts/smooch-sans";
import { Stack } from "expo-router";
import * as SplashScreen from "expo-splash-screen";
import { useEffect } from "react";

import { AuthProvider, useAuth } from "@/context/auth-context";
import { ThemeProvider } from "@/theme/theme-provider";

SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
	const [fontsLoaded] = useFonts({
		SmoochSans_400Regular,
		SmoochSans_500Medium,
		SmoochSans_600SemiBold,
		SmoochSans_700Bold,
	});

	return (
		<ThemeProvider>
			<AuthProvider>
				<RootNavigator fontsLoaded={fontsLoaded} />
			</AuthProvider>
		</ThemeProvider>
	);
}

function RootNavigator({ fontsLoaded }: { fontsLoaded: boolean }) {
	const { isReady } = useAuth();
	const ready = fontsLoaded && isReady;

	useEffect(() => {
		if (ready) {
			SplashScreen.hideAsync().catch(() => {});
		}
	}, [ready]);

	if (!ready) return null;

	return (
		<Stack screenOptions={{ headerShown: false, animation: "none" }}>
			<Stack.Screen
				name="(auth)"
				options={{ headerShown: false, animation: "none" }}
			/>
			<Stack.Screen
				name="(protected)"
				options={{ headerShown: false, animation: "none" }}
			/>
		</Stack>
	);
}
