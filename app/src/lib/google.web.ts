// Web stub — Metro picks this file over google.ts on web so the native
// Nitro module is never imported in the browser bundle.
export function configureGoogleSignIn(): void {}

export async function signInWithGoogleNative(): Promise<string | null> {
	throw new Error("Native Google Sign-In is unavailable on web.");
}
