import { client } from "./api";

export type AuthUser = {
	id: string;
	email: string;
	display_name: string;
};

export type TokenPair = {
	access_token: string;
	refresh_token: string;
	expires_in: number;
	user: AuthUser;
};

export async function register(
	email: string,
	password: string,
	displayName: string,
): Promise<TokenPair> {
	const { data } = await client.post<TokenPair>("/auth/register", {
		email,
		password,
		display_name: displayName,
	});
	return data;
}

export async function login(
	email: string,
	password: string,
): Promise<TokenPair> {
	const { data } = await client.post<TokenPair>("/auth/login", {
		email,
		password,
	});
	return data;
}

export async function refreshTokens(refreshToken: string): Promise<TokenPair> {
	const { data } = await client.post<TokenPair>("/auth/refresh", {
		refresh_token: refreshToken,
	});
	return data;
}

export async function logoutRemote(refreshToken: string): Promise<void> {
	await client.post("/auth/logout", { refresh_token: refreshToken });
}
