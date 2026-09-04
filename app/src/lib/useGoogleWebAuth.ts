import {
	makeRedirectUri,
	ResponseType,
	useAuthRequest,
	useAutoDiscovery,
} from "expo-auth-session";
import * as Crypto from "expo-crypto";
import * as WebBrowser from "expo-web-browser";
import { useMemo, useState } from "react";

import { GOOGLE_WEB_CLIENT_ID } from "./google-ids";

WebBrowser.maybeCompleteAuthSession();

type WebGoogleResult = {
	webReady: boolean;
	webError: string | null;
	promptWebGoogle: () => Promise<string | null>;
};

export function useGoogleWebAuth(): WebGoogleResult {
	const discovery = useAutoDiscovery("https://accounts.google.com");
	// Google mandates a nonce for response_type=id_token and echoes it in
	// the id_token's nonce claim. One per page load is enough here — the
	// backend verifies signature/aud/iss/expiry, not the nonce.
	const nonce = useMemo(() => Crypto.randomUUID(), []);
	const [request, , promptAsync] = useAuthRequest(
		{
			clientId: GOOGLE_WEB_CLIENT_ID,
			scopes: ["openid", "profile", "email"],
			redirectUri: makeRedirectUri(),
			responseType: ResponseType.IdToken,
			usePKCE: false,
			extraParams: { nonce },
		},
		discovery,
	);
	const [webError, setWebError] = useState<string | null>(null);

	const promptWebGoogle = async (): Promise<string | null> => {
		setWebError(null);
		if (!request) return null;
		const result = await promptAsync();
		if (result.type === "success") {
			const idToken = (result.params as { id_token?: string }).id_token;
			if (!idToken) {
				setWebError("Google did not return an ID token.");
				return null;
			}
			return idToken;
		}
		if (result.type === "error") {
			setWebError("Google sign-in failed. Please try again.");
		}
		return null;
	};

	return { webReady: request !== null, webError, promptWebGoogle };
}
