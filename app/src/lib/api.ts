import axios, { type AxiosError, type InternalAxiosRequestConfig } from "axios";
import { Platform } from "react-native";

const API_PORT = 8081;

// Android emulator can't reach the host machine via `localhost` — it maps
// `10.0.2.2` to the host loopback instead. iOS simulator and web both share
// the host's network namespace directly. A physical device needs the host's
// LAN IP, which isn't knowable here — override via EXPO_PUBLIC_API_BASE_URL.
export function resolveBaseUrl(): string {
	const override = process.env.EXPO_PUBLIC_API_BASE_URL;
	if (override) return override;
	const host = Platform.OS === "android" ? "10.0.2.2" : "localhost";
	return `http://${host}:${API_PORT}`;
}

export const client = axios.create({
	baseURL: resolveBaseUrl(),
	headers: { "Content-Type": "application/json" },
});

export class ApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.status = status;
	}
}

let inMemoryAccessToken: string | null = null;
let refreshHandler: (() => Promise<string | null>) | null = null;
let onAuthInvalid: (() => void) | null = null;
let inflightRefresh: Promise<string | null> | null = null;

export function setAccessToken(token: string | null): void {
	inMemoryAccessToken = token;
}

export function getAccessToken(): string | null {
	return inMemoryAccessToken;
}

export function getAuthHeaders(): Record<string, string> {
	return inMemoryAccessToken
		? { Authorization: `Bearer ${inMemoryAccessToken}` }
		: {};
}

export function setRefreshHandler(
	fn: (() => Promise<string | null>) | null,
): void {
	refreshHandler = fn;
}

export function setOnAuthInvalid(fn: (() => void) | null): void {
	onAuthInvalid = fn;
}

function isAuthEndpoint(url?: string): boolean {
	return !!url && url.startsWith("/auth/");
}

type RetriableConfig = InternalAxiosRequestConfig & { _retried?: boolean };

client.interceptors.request.use((config) => {
	if (inMemoryAccessToken && !isAuthEndpoint(config.url)) {
		config.headers = config.headers ?? {};
		if (!config.headers.Authorization) {
			config.headers.Authorization = `Bearer ${inMemoryAccessToken}`;
		}
	}
	return config;
});

client.interceptors.response.use(
	(response) => response,
	async (error: AxiosError<{ error?: string }>) => {
		const original = error.config as RetriableConfig | undefined;
		const status = error.response?.status ?? 0;

		if (
			status === 401 &&
			original &&
			!original._retried &&
			!isAuthEndpoint(original.url) &&
			refreshHandler
		) {
			original._retried = true;
			try {
				if (!inflightRefresh) {
					inflightRefresh = refreshHandler().finally(() => {
						inflightRefresh = null;
					});
				}
				const next = await inflightRefresh;
				if (next) {
					original.headers = original.headers ?? {};
					original.headers.Authorization = `Bearer ${next}`;
					return client.request(original);
				}
			} catch {
				// Refresh itself failed — fall through to invalid-session handling.
			}
			onAuthInvalid?.();
		}

		const message = error.response?.data?.error ?? error.message;
		throw new ApiError(status, message);
	},
);
