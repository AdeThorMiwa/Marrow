import { router } from "expo-router";
import { useState } from "react";
import { Pressable, useWindowDimensions, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

import { Button, Text, TextInput } from "@/components/ui";
import { useAuth } from "@/context/auth-context";
import { useTheme } from "@/theme/theme-provider";

const DESKTOP_BREAKPOINT = 768;

type Mode = "login" | "register";

function isValidEmail(value: string): boolean {
	return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
}

export default function LoginScreen() {
	const theme = useTheme();
	const { width } = useWindowDimensions();
	const isDesktop = width >= DESKTOP_BREAKPOINT;
	const horizontalInset = isDesktop ? theme.spacing.lg : theme.spacing.md;
	const columnBorderWidth = isDesktop ? theme.hairlineWidth : 0;

	const { login, register, isLoading } = useAuth();
	const [mode, setMode] = useState<Mode>("login");
	const [email, setEmail] = useState("");
	const [password, setPassword] = useState("");
	const [displayName, setDisplayName] = useState("");
	const [formError, setFormError] = useState<string | null>(null);

	const switchMode = (next: Mode) => {
		setMode(next);
		setFormError(null);
	};

	const onSubmit = async () => {
		const trimmedEmail = email.trim();
		const trimmedDisplayName = displayName.trim();
		if (!isValidEmail(trimmedEmail)) {
			setFormError("Enter a valid email address.");
			return;
		}
		if (password.length === 0) {
			setFormError("Enter your password.");
			return;
		}
		if (mode === "register" && trimmedDisplayName.length === 0) {
			setFormError("Choose a display name.");
			return;
		}
		setFormError(null);
		try {
			if (mode === "login") {
				await login(trimmedEmail, password);
			} else {
				await register(trimmedEmail, password, trimmedDisplayName);
			}
			router.replace("/");
		} catch (e) {
			setFormError(
				e instanceof Error
					? e.message
					: "Something went wrong. Please try again.",
			);
		}
	};

	return (
		<SafeAreaView style={{ flex: 1, backgroundColor: theme.colors.background }}>
			<View
				style={
					isDesktop
						? {
								flex: 1,
								width: "50%",
								maxWidth: theme.maxContentWidth,
								alignSelf: "center",
							}
						: { flex: 1 }
				}
			>
				<View
					style={{
						flex: 1,
						borderLeftWidth: columnBorderWidth,
						borderRightWidth: columnBorderWidth,
						borderColor: theme.colors.divider,
					}}
				>
					<View
						style={{
							paddingHorizontal: horizontalInset,
							paddingVertical: theme.spacing.md,
						}}
					>
						<Text variant="itemTitle">Marrow</Text>
					</View>
					<View
						style={{
							height: theme.hairlineWidth,
							backgroundColor: theme.colors.divider,
						}}
					/>

					<View
						style={{
							paddingHorizontal: horizontalInset,
							paddingVertical: theme.spacing.lg,
							gap: theme.spacing.md,
						}}
					>
						<View style={{ flexDirection: "row", gap: theme.spacing.md }}>
							<ModeTab
								label="Log in"
								active={mode === "login"}
								onPress={() => switchMode("login")}
							/>
							<ModeTab
								label="Create account"
								active={mode === "register"}
								onPress={() => switchMode("register")}
							/>
						</View>

						<Text variant="body" tone="secondary">
							{mode === "login"
								? "Welcome back. Log in to pick up your feed."
								: "One account for everything. Your feed is waiting."}
						</Text>

						<TextInput
							label="Email"
							placeholder="you@example.com"
							value={email}
							onChangeText={setEmail}
							autoCapitalize="none"
							autoCorrect={false}
							keyboardType="email-address"
							textContentType="emailAddress"
							returnKeyType="next"
						/>
						{mode === "register" ? (
							<TextInput
								label="Display name"
								placeholder="What should we call you?"
								value={displayName}
								onChangeText={setDisplayName}
								autoCapitalize="words"
								autoCorrect={false}
								textContentType="nickname"
								returnKeyType="next"
							/>
						) : null}
						<TextInput
							label="Password"
							placeholder={
								mode === "login" ? "Your password" : "Pick a password"
							}
							value={password}
							onChangeText={setPassword}
							secureTextEntry
							autoCapitalize="none"
							autoCorrect={false}
							textContentType={mode === "login" ? "password" : "newPassword"}
							returnKeyType="done"
							onSubmitEditing={() => void onSubmit()}
						/>

						{formError ? (
							<Text variant="body" tone="secondary">
								{formError}
							</Text>
						) : null}

						<Button onPress={() => void onSubmit()} disabled={isLoading}>
							{isLoading
								? "Please wait…"
								: mode === "login"
									? "Log in"
									: "Create account"}
						</Button>
					</View>
				</View>
			</View>
		</SafeAreaView>
	);
}

function ModeTab({
	label,
	active,
	onPress,
}: {
	label: string;
	active: boolean;
	onPress: () => void;
}) {
	const theme = useTheme();
	return (
		<Pressable
			accessibilityRole="tab"
			accessibilityState={{ selected: active }}
			onPress={onPress}
			style={{
				paddingBottom: theme.spacing.xs,
				borderBottomWidth: active
					? theme.borderWidthError
					: theme.hairlineWidth,
				borderColor: theme.colors.ink,
			}}
		>
			<Text variant="label" tone={active ? "primary" : "secondary"}>
				{label}
			</Text>
		</Pressable>
	);
}
