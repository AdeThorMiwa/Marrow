import { Redirect, Stack } from "expo-router";

import { useAuth } from "@/context/auth-context";

export default function AuthLayout() {
	const { isLoggedIn, isReady } = useAuth();

	if (!isReady) {
		return null;
	}

	if (isLoggedIn) {
		return <Redirect href="/" />;
	}

	return (
		<Stack screenOptions={{ headerShown: false, animation: "none" }}>
			<Stack.Screen name="login" />
		</Stack>
	);
}
