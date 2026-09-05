import { Platform } from "react-native";
import {
	GoogleOneTapSignIn,
	isCancelledResponse,
	isNoSavedCredentialFoundResponse,
	isSuccessResponse,
} from "react-native-nitro-google-signin";

import { GOOGLE_IOS_CLIENT_ID, GOOGLE_WEB_CLIENT_ID } from "./google-ids";

let configured = false;

// Must run before any other Nitro call. No-op after the first call.
export function configureGoogleSignIn(): void {
	if (configured) return;
	GoogleOneTapSignIn.configure({
		webClientId: GOOGLE_WEB_CLIENT_ID,
		iosClientId: GOOGLE_IOS_CLIENT_ID,
	});
	configured = true;
}

// Returns the Google id_token, or null when the user dismisses the UI.
// Throws on real failures (Play Services, misconfiguration, network).
export async function signInWithGoogleNative(): Promise<string | null> {
	configureGoogleSignIn();
	if (Platform.OS === "android") {
		await GoogleOneTapSignIn.checkPlayServices();
	}
	let response = await GoogleOneTapSignIn.signIn();
	if (isNoSavedCredentialFoundResponse(response)) {
		response = await GoogleOneTapSignIn.createAccount();
	}
	if (isNoSavedCredentialFoundResponse(response)) {
		response = await GoogleOneTapSignIn.presentExplicitSignIn();
	}
	if (isCancelledResponse(response)) {
		return null;
	}
	if (isSuccessResponse(response)) {
		return response.data.idToken ?? null;
	}
	return null;
}
