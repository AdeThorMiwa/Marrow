import AsyncStorage from "@react-native-async-storage/async-storage";
import * as SecureStore from "expo-secure-store";
import {
	createContext,
	type ReactNode,
	use,
	useCallback,
	useEffect,
	useMemo,
	useRef,
	useState,
} from "react";
import {
	ApiError,
	setAccessToken,
	setOnAuthInvalid,
	setRefreshHandler,
} from "@/lib/api";
import {
	type AuthUser,
	login as loginRequest,
	logoutRemote,
	refreshTokens,
	register as registerRequest,
	type TokenPair,
} from "@/lib/auth";

const SECURE_TOKENS_KEY = "marrow.auth.tokens.v1";
const USER_STORAGE_KEY = "marrow.auth.user.v1";
// SecureStore is native-only (unavailable on web) — tokens fall back to
// AsyncStorage there so web dev still works.
const TOKENS_FALLBACK_KEY = "marrow.auth.tokens.fallback.v1";

type StoredTokens = {
	access_token: string;
	refresh_token: string;
	expires_at: number;
};

// Refresh a minute before the access token actually dies so in-flight
// requests never race expiry.
const REFRESH_SKEW_MS = 60_000;

async function saveTokens(
	tokens: StoredTokens,
	secureAvailable: boolean,
): Promise<void> {
	const serialized = JSON.stringify(tokens);
	if (secureAvailable) {
		await SecureStore.setItemAsync(SECURE_TOKENS_KEY, serialized);
	} else {
		await AsyncStorage.setItem(TOKENS_FALLBACK_KEY, serialized);
	}
}

async function loadTokens(
	secureAvailable: boolean,
): Promise<StoredTokens | null> {
	try {
		const serialized = secureAvailable
			? await SecureStore.getItemAsync(SECURE_TOKENS_KEY)
			: await AsyncStorage.getItem(TOKENS_FALLBACK_KEY);
		if (!serialized) return null;
		const parsed = JSON.parse(serialized) as StoredTokens;
		if (!parsed.access_token || !parsed.refresh_token || !parsed.expires_at)
			return null;
		return parsed;
	} catch {
		return null;
	}
}

async function deleteTokens(secureAvailable: boolean): Promise<void> {
	try {
		if (secureAvailable) {
			await SecureStore.deleteItemAsync(SECURE_TOKENS_KEY);
		} else {
			await AsyncStorage.removeItem(TOKENS_FALLBACK_KEY);
		}
	} catch {
		// Clearing best-effort — a stale token here just means the next launch
		// attempts one doomed refresh before landing logged-out.
	}
}

function isExpired(tokens: StoredTokens): boolean {
	return Date.now() >= tokens.expires_at - REFRESH_SKEW_MS;
}

export type AuthContextValue = {
	user: AuthUser | null;
	isLoggedIn: boolean;
	isReady: boolean;
	isLoading: boolean;
	error: string | null;
	login: (email: string, password: string) => Promise<void>;
	register: (
		email: string,
		password: string,
		displayName: string,
	) => Promise<void>;
	logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
	const [user, setUser] = useState<AuthUser | null>(null);
	const [isReady, setIsReady] = useState(false);
	const [isLoading, setIsLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);

	// check if secure store is available, and store the result in state
	const [secureAvailable, setSecureAvailable] = useState<boolean | null>(null);
	const [secureAvailableResolved, setSecureAvailableResolved] = useState(false);
	// Mirrors for the api.ts refresh handler — it runs outside React render
	// and must never close over a stale token.
	const refreshTokenRef = useRef<string | null>(null);

	const clearLocalSession = useCallback(async () => {
		refreshTokenRef.current = null;
		setAccessToken(null);
		setUser(null);
		await Promise.all([
			deleteTokens(secureAvailable ?? false),
			AsyncStorage.removeItem(USER_STORAGE_KEY).catch(() => {}),
		]);
	}, [secureAvailable]);

	const persistPair = useCallback(
		async (pair: TokenPair) => {
			const tokens: StoredTokens = {
				access_token: pair.access_token,
				refresh_token: pair.refresh_token,
				expires_at: Date.now() + pair.expires_in * 1000,
			};
			refreshTokenRef.current = tokens.refresh_token;
			setAccessToken(tokens.access_token);
			setUser(pair.user);
			await Promise.all([
				saveTokens(tokens, secureAvailable ?? false),
				AsyncStorage.setItem(USER_STORAGE_KEY, JSON.stringify(pair.user)),
			]);
		},
		[secureAvailable],
	);

	const login = useCallback(
		async (email: string, password: string) => {
			setIsLoading(true);
			setError(null);
			try {
				await persistPair(await loginRequest(email, password));
			} catch (e) {
				const message =
					e instanceof ApiError ? e.message : "Login failed. Please try again.";
				setError(message);
				throw new Error(message);
			} finally {
				setIsLoading(false);
			}
		},
		[persistPair],
	);

	const register = useCallback(
		async (email: string, password: string, displayName: string) => {
			setIsLoading(true);
			setError(null);
			try {
				await persistPair(await registerRequest(email, password, displayName));
			} catch (e) {
				const message =
					e instanceof ApiError
						? e.message
						: "Registration failed. Please try again.";
				setError(message);
				throw new Error(message);
			} finally {
				setIsLoading(false);
			}
		},
		[persistPair],
	);

	const logout = useCallback(async () => {
		const refreshToken = refreshTokenRef.current;
		if (refreshToken) {
			try {
				await logoutRemote(refreshToken);
			} catch {
				// Server-side logout is the best option, local session is
				// cleared regardless so the user always lands logged-out.
			}
		}
		await clearLocalSession();
	}, [clearLocalSession]);

	useEffect(() => {
		(async () => {
			let available = false;
			try {
				available = await SecureStore.isAvailableAsync();
			} catch {
				available = false;
			}
			setSecureAvailable(available);
			setSecureAvailableResolved(true);
		})();
	}, []);

	// Restore on launch + wire the api.ts auto-refresh plumbing.
	useEffect(() => {
		if (!secureAvailableResolved) return;

		let cancelled = false;

		setRefreshHandler(async () => {
			const current =
				refreshTokenRef.current ??
				(await loadTokens(secureAvailable ?? false))?.refresh_token ??
				null;
			if (!current) return null;
			try {
				const pair = await refreshTokens(current);
				if (cancelled) return pair.access_token;
				await persistPair(pair);
				return pair.access_token;
			} catch {
				if (!cancelled) await clearLocalSession();
				return null;
			}
		});
		setOnAuthInvalid(() => {
			if (!cancelled) void clearLocalSession();
		});

		(async () => {
			try {
				const [tokens, storedUser] = await Promise.all([
					loadTokens(secureAvailable ?? false),
					AsyncStorage.getItem(USER_STORAGE_KEY).catch(() => null),
				]);
				if (cancelled) return;

				if (tokens) {
					if (isExpired(tokens)) {
						try {
							await persistPair(await refreshTokens(tokens.refresh_token));
						} catch {
							await clearLocalSession();
						}
					} else {
						refreshTokenRef.current = tokens.refresh_token;
						setAccessToken(tokens.access_token);
						if (storedUser) {
							try {
								setUser(JSON.parse(storedUser) as AuthUser);
							} catch {
								setUser(null);
							}
						}
					}
				}
			} catch {
				// A corrupt store reads as logged-out — never crash launch on it.
				if (!cancelled) await clearLocalSession();
			} finally {
				if (!cancelled) setIsReady(true);
			}
		})();

		return () => {
			cancelled = true;
			setRefreshHandler(null);
			setOnAuthInvalid(null);
		};
	}, [
		clearLocalSession,
		persistPair,
		secureAvailable,
		secureAvailableResolved,
	]);

	const value = useMemo<AuthContextValue>(
		() => ({
			user,
			isLoggedIn: user !== null,
			isReady,
			isLoading,
			error,
			login,
			register,
			logout,
		}),
		[user, isReady, isLoading, error, login, register, logout],
	);

	return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
	const ctx = use(AuthContext);
	if (!ctx) {
		throw new Error("useAuth must be used within an AuthProvider");
	}
	return ctx;
}
