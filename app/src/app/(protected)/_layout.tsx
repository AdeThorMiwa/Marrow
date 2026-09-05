import { Redirect, Stack } from "expo-router";

import { useAuth } from "@/context/auth-context";

export default function ProtectedLayout() {
	const { isLoggedIn, isReady } = useAuth();
	if (!isReady) {
		return null;
	}

	if (!isLoggedIn) {
		return <Redirect href="/login" />;
	}

	return <Stack screenOptions={{ headerShown: false, animation: "none" }} />;
}
