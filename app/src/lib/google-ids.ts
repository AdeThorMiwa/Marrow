// Manual Google Cloud Console IDs (no Firebase files).
// Backend `google_client_id` must equal GOOGLE_WEB_CLIENT_ID — the `aud`
export const GOOGLE_WEB_CLIENT_ID =
	process.env.EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID ??
	"1030635231460-pu03226pqi8dqhh210pm3cbiooi41bl2.apps.googleusercontent.com";

export const GOOGLE_IOS_CLIENT_ID =
	process.env.EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID ??
	"1030635231460-00aptke8mj8mdindfrdctae40t5mrs33.apps.googleusercontent.com";
