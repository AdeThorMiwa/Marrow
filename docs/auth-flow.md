Below is a cleaned-up step-by-step PRD/implementation guide based on the transcript, with the Expo Router concepts, file structure, state management, persistence, and splash-screen handling turned into an actionable document.

PRD: Authentication Flow for a React Native Expo App
1. Overview
Objective

Implement a robust authentication flow in a React Native application built with Expo + Expo Router.

The application should support two primary states:

Logged out

User is shown the login/register screen.
Protected application routes cannot be accessed.
Deep links into protected routes must redirect to the login screen.

Logged in

User can access all protected application routes.
User can log out from within the application.
Authentication state should persist across application launches.
Core Requirements

The authentication implementation must:

Protect all authenticated routes.
Redirect unauthenticated users to /login.
Redirect authenticated users into the protected application.
Work correctly with Expo Router's file-based routing.
Prevent users from bypassing authentication through deep links.
Avoid navigation animations during login/logout transitions.
Persist authentication state using AsyncStorage.
Restore authentication state when the app launches.
Prevent redirects before persisted authentication state has been loaded.
Keep the splash screen visible while the initial authentication state is being determined.
Use router.replace() rather than router.push() for authentication transitions.
2. Why Authentication Requires Special Handling in Expo Router

With traditional React Navigation, it is common to conditionally define which navigators/screens are available based on authentication state.

Expo Router uses file-based routing, so routes are defined by the application's file structure.

Instead of conditionally creating/removing screens, the authentication flow should use redirects.

Conceptually:

App starts
   ↓
Determine authentication state
   ↓
Is user authenticated?
   ├── Yes → Protected application
   └── No  → Login screen


The authentication check should happen before protected screens are rendered.

3. Target Route Structure

The application should be reorganized so that protected routes live inside a grouping folder.

Recommended structure:

app/
├── _layout.tsx
├── login.tsx
│
└── (protected)/
    ├── _layout.tsx
    └── (tabs)/
        ├── _layout.tsx
        ├── index.tsx
        ├── ...
        └── fourth-screen.tsx
│
context/
└── auth-context.tsx


The important part is:

app/
├── _layout.tsx
├── login.tsx
└── (protected)/
    └── ...


The (protected) folder is a route group.

Because it is surrounded by parentheses, it organizes routes without adding protected to the URL/path.

For example:

app/(protected)/(tabs)/index.tsx


still resolves to:

/


rather than:

/protected/tabs

4. Navigation Architecture

Expo Router loads the navigation tree from the root downward.

For a route such as:

app/(protected)/(tabs)/index.tsx


the approximate rendering order is:

app/_layout.tsx
      ↓
app/(protected)/_layout.tsx
      ↓
app/(protected)/(tabs)/_layout.tsx
      ↓
app/(protected)/(tabs)/index.tsx


Each layout contributes to the navigation tree.

This explains why the root layout should return a navigator rather than a redirect.

Important Constraint

The root layout should establish the navigation tree.

The authentication redirect should therefore live in the protected route group's layout:

app/(protected)/_layout.tsx


This layout is executed before the protected screens underneath it are rendered.

5. Root Layout
Requirement

The root layout must:

Return the application's root navigator.
Provide the authentication context to the entire application.
Configure the login and protected navigators.
Disable unwanted navigation animations.

Conceptually:

<AuthProvider>
  <Stack>
    <Stack.Screen
      name="login"
      options={{
        headerShown: false,
        animation: "none",
      }}
    />

    <Stack.Screen
      name="(protected)"
      options={{
        headerShown: false,
        animation: "none",
      }}
    />
  </Stack>
</AuthProvider>


The exact configuration may vary depending on the existing application's navigator structure.

Acceptance Criteria
The root layout returns a navigator.
AuthProvider wraps the complete application.
Authentication state is accessible from every route.
Login and protected routes do not show unwanted headers.
Login/logout transitions do not use the default slide animation.
6. Login Screen

Create:

app/login.tsx


The login screen is responsible for:

Displaying login/register UI.
Calling the authentication context's login() function.
Navigating the user into the protected application after successful login.

Basic flow:

User taps Login
      ↓
auth.login()
      ↓
isLoggedIn = true
      ↓
router.replace("/")


Use router.replace() instead of router.push().

Why replace()?

With:

router.push("/");


the login screen remains in the navigation history.

The user could potentially swipe/back-navigate to the login screen.

Instead:

router.replace("/");


replaces the current route.

This produces the desired authentication behavior:

Login → App


rather than:

Login → App → Login (in navigation history)

7. Authentication Context

Create:

context/auth-context.tsx


The context is responsible for owning and exposing authentication state.

Required State

The initial implementation needs:

isLoggedIn: boolean
isReady: boolean


It should also expose:

login(): void
logout(): void


Conceptually:

type AuthState = {
  isLoggedIn: boolean;
  isReady: boolean;
  login: () => void;
  logout: () => void;
};


In a real production application, this state would likely contain more information, such as:

type AuthState = {
  isLoggedIn: boolean;
  isReady: boolean;
  accessToken?: string;
  refreshToken?: string;
  expiresAt?: number;
};


The exact shape should depend on the backend authentication system.

8. Auth Provider

The AuthProvider should:

Maintain authentication state.
Expose login().
Expose logout().
Persist authentication state.
Restore authentication state when the app starts.
Track whether initialization has completed.
Control the splash screen.

Conceptually:

const [isLoggedIn, setIsLoggedIn] = useState(false);
const [isReady, setIsReady] = useState(false);


The provider then exposes:

<AuthContext.Provider
  value={{
    isLoggedIn,
    isReady,
    login,
    logout,
  }}
>
  {children}
</AuthContext.Provider>

9. Protecting Routes

Create:

app/(protected)/_layout.tsx


This layout should read authentication state from the context.

Conceptual implementation:

const { isLoggedIn, isReady } = useContext(AuthContext);

if (!isReady) {
  return null;
}

if (!isLoggedIn) {
  return <Redirect href="/login" />;
}

return <Stack />;

Order Is Important

The checks must happen in this order:

Is authentication state ready?
       ↓
No → render nothing
       ↓
Yes
       ↓
Is user logged in?
       ↓
No → Redirect to /login
       ↓
Yes → render protected navigator


Do not immediately redirect based on isLoggedIn before persisted state has been loaded.

10. Why isReady Is Necessary

At application startup:

const [isLoggedIn, setIsLoggedIn] = useState(false);


initially produces:

isLoggedIn = false


But that does not necessarily mean the user is logged out.

The app may simply not have loaded the persisted authentication state yet.

Without an isReady flag:

App launches
   ↓
isLoggedIn = false
   ↓
Redirect to /login
   ↓
AsyncStorage finishes loading
   ↓
isLoggedIn = true


The user may briefly be redirected to the login page even though they are actually authenticated.

Therefore:

if (!isReady) {
  return null;
}


must happen before the authentication redirect.

11. Add AsyncStorage Persistence

Install AsyncStorage:

npx expo install @react-native-async-storage/async-storage


Then restart the bundler if necessary.

AsyncStorage stores string key/value pairs, so the authentication state must be serialized.

Define a storage key:

const AUTH_STORAGE_KEY = "@auth_state";


Persist state using:

await AsyncStorage.setItem(
  AUTH_STORAGE_KEY,
  JSON.stringify(authState)
);


Read it using:

const value = await AsyncStorage.getItem(AUTH_STORAGE_KEY);


Then deserialize:

const authState = JSON.parse(value);

12. Persist Login

When login() executes:

login()
  ↓
set isLoggedIn = true
  ↓
persist state
  ↓
router.replace("/")


Conceptually:

const login = async () => {
  const nextState = {
    isLoggedIn: true,
  };

  setIsLoggedIn(true);

  await AsyncStorage.setItem(
    AUTH_STORAGE_KEY,
    JSON.stringify(nextState)
  );
};


In a real application, this should happen after successful authentication with the backend.

13. Persist Logout

When logout() executes:

logout()
  ↓
set isLoggedIn = false
  ↓
persist updated state
  ↓
navigate to login


Conceptually:

const logout = async () => {
  const nextState = {
    isLoggedIn: false,
  };

  setIsLoggedIn(false);

  await AsyncStorage.setItem(
    AUTH_STORAGE_KEY,
    JSON.stringify(nextState)
  );
};


If the application stores tokens, logout should also invalidate/remove those credentials as appropriate.

14. Restore Authentication on Launch

Use useEffect() with an empty dependency array:

useEffect(() => {
  const getAuthFromStorage = async () => {
    try {
      const value = await AsyncStorage.getItem(
        AUTH_STORAGE_KEY
      );

      if (value) {
        const storedState = JSON.parse(value);

        setIsLoggedIn(storedState.isLoggedIn);
      }
    } catch (error) {
      console.error("Failed to restore auth state", error);
    } finally {
      setIsReady(true);
    }
  };

  getAuthFromStorage();
}, []);


The important point is that the function passed directly to useEffect() should remain synchronous.

Instead of:

useEffect(async () => {
  ...
}, []);


define an async function inside the effect and call it.

15. Authentication Initialization Lifecycle

The complete startup sequence should now be:

App launches
     ↓
Root layout renders
     ↓
AuthProvider initializes
     ↓
isReady = false
     ↓
Load authentication state from AsyncStorage
     ↓
Authentication state restored
     ↓
isReady = true
     ↓
Protected layout renders
     ↓
Check isLoggedIn
     ├── true  → Protected app
     └── false → /login

16. Splash Screen Handling

Without splash-screen handling, a slow authentication initialization could produce:

Splash screen
     ↓
Splash disappears
     ↓
Blank screen
     ↓
Authentication state loads
     ↓
App/login appears


This is undesirable.

Instead, keep the native splash screen visible until authentication initialization is complete.

Expo Router includes the Expo splash-screen functionality.

At the top level of the authentication initialization, prevent automatic hiding:

SplashScreen.preventAutoHideAsync();


Then hide the splash screen when the authentication state becomes ready.

useEffect(() => {
  if (isReady) {
    SplashScreen.hideAsync();
  }
}, [isReady]);


The desired lifecycle becomes:

App launches
     ↓
Splash screen remains visible
     ↓
Load authentication state
     ↓
isReady = true
     ↓
Hide splash screen
     ↓
Show correct screen

17. Simulating Slow Authentication Initialization

For testing, temporarily add a delay to the initialization process:

await new Promise(resolve =>
  setTimeout(resolve, 1000)
);


This allows the team to verify that:

The splash screen remains visible.
No login screen flashes before authentication is restored.
No protected screen flashes before authentication is restored.
The correct route appears once initialization finishes.

Remove the artificial delay before production.

18. Logout UI

Add a logout button somewhere inside the protected application, for example in a settings/profile/fourth tab.

The button should:

User taps Logout
       ↓
auth.logout()
       ↓
isLoggedIn = false
       ↓
Persist state
       ↓
router.replace("/login")


Even though changing isLoggedIn will cause the protected layout's redirect to run automatically, explicitly navigating to /login is recommended for clarity and predictable UX.

19. Deep Linking Requirements

Deep linking must not allow unauthenticated access to protected screens.

For example, suppose the user opens:

myapp://some-protected-screen


while logged out.

The flow must be:

Deep link requested
       ↓
Protected layout executes
       ↓
isReady?
       ├── No → wait
       └── Yes
             ↓
         isLoggedIn?
          ├── No → /login
          └── Yes → requested screen


This is one of the main reasons the authentication check belongs in the protected layout rather than individual screens.

20. Navigation Animation

Authentication transitions generally should not use the default navigation slide animation.

Configure the relevant screens/navigators with:

animation: "none"


This should apply to:

The login screen.
The protected navigator.

The resulting experience is:

Login
  ↓
instant transition
  ↓
App


and:

App
  ↓
instant transition
  ↓
Login


rather than appearing as normal page navigation.

21. Final File Responsibilities
app/_layout.tsx

Responsible for:

Creating the root navigation tree.
Wrapping the application with AuthProvider.
Registering the login route.
Registering the protected route group.
Configuring headers.
Configuring authentication transition animations.

It should not perform the authentication redirect.

app/login.tsx

Responsible for:

Login/register UI.
Calling login().
Navigating to the protected application after successful login.
app/(protected)/_layout.tsx

Responsible for:

Checking isReady.
Checking isLoggedIn.
Redirecting unauthenticated users.
Returning the protected navigator for authenticated users.
context/auth-context.tsx

Responsible for:

Authentication state.
login().
logout().
AsyncStorage persistence.
Restoring authentication state.
isReady.
Splash-screen lifecycle.
Protected screens

Responsible only for application functionality.

They should generally not need to individually check whether the user is authenticated because the protected layout already guards the entire route tree.

22. Recommended Implementation Order

Implement the feature in the following sequence.

Step 1 — Create the protected route group

Move existing authenticated screens into:

app/(protected)/

Step 2 — Create the new root layout

Create/update:

app/_layout.tsx


Make sure it returns the root Stack.

Step 3 — Create the protected layout

Create:

app/(protected)/_layout.tsx


Initially return the protected Stack.

Step 4 — Create the login route

Create:

app/login.tsx


Add temporary login UI.

Step 5 — Add authentication context

Create:

context/auth-context.tsx


Add:

isLoggedIn
isReady
login()
logout()
Step 6 — Wrap the application

Wrap the root navigator with:

<AuthProvider>
  ...
</AuthProvider>

Step 7 — Protect the route group

In:

app/(protected)/_layout.tsx


add:

if (!isReady) {
  return null;
}

if (!isLoggedIn) {
  return <Redirect href="/login" />;
}

Step 8 — Connect login

From login.tsx:

login()
→ router.replace("/")

Step 9 — Connect logout

From a protected screen:

logout()
→ router.replace("/login")

Step 10 — Add AsyncStorage

Persist the authentication state after login/logout.

Step 11 — Restore state on launch

Load the stored state inside the AuthProvider.

Step 12 — Add isReady

Prevent protected-route redirects until storage initialization completes.

Step 13 — Add splash-screen handling

Keep the splash screen visible until:

isReady === true

Step 14 — Test deep links

Verify unauthenticated users cannot access protected routes directly.

Step 15 — Test slow initialization

Temporarily add an artificial delay and verify that no blank/incorrect screen is displayed.

23. Acceptance Criteria

The feature is complete when all of the following are true:

 The application has a distinct logged-in and logged-out experience.
 Protected screens live under a protected route group.
 The root layout always returns a navigator.
 Authentication logic exists in the protected layout.
 Unauthenticated users are redirected to /login.
 Authenticated users can access protected routes.
 Deep links to protected screens are guarded.
 Login changes authentication state.
 Login uses router.replace().
 Logout changes authentication state.
 Logout uses router.replace().
 Authentication state persists across app restarts.
 Authentication state is restored from AsyncStorage.
 isReady prevents premature redirects.
 The splash screen remains visible while auth state initializes.
 The splash screen hides after initialization completes.
 Login/logout transitions do not use an unnecessary slide animation.
 No protected screen is briefly visible to an unauthenticated user.
 No login screen briefly flashes for an already-authenticated user.
24. Production Considerations

The transcript uses a simple Boolean:

isLoggedIn: boolean


This is useful for demonstrating the routing architecture, but a production authentication system will usually need more.

A production auth state might look like:

type AuthState = {
  isReady: boolean;
  isLoggedIn: boolean;
  accessToken: string | null;
  refreshToken: string | null;
  expiresAt: number | null;
  user: User | null;
};


Additional production requirements may include:

Secure token storage rather than plain AsyncStorage for sensitive credentials.
Refresh-token handling.
Token expiration detection.
Automatic token refresh.
Backend session validation.
Handling revoked sessions.
Logout cleanup.
Loading/error states.
Handling network failures.
Account deletion.
Password reset.
Email verification.
Social authentication.
Biometric authentication where appropriate.

The routing architecture remains fundamentally the same:

                ┌─────────────────┐
                │   App launches  │
                └────────┬────────┘
                         ↓
                ┌─────────────────┐
                │ AuthProvider    │
                │ initializes     │
                └────────┬────────┘
                         ↓
                ┌─────────────────┐
                │ Load persisted  │
                │ auth/session     │
                └────────┬────────┘
                         ↓
                    isReady = true
                         ↓
                ┌─────────────────┐
                │ Protected       │
                │ Layout          │
                └────────┬────────┘
                         ↓
                 ┌───────────────┐
                 │ Logged in?    │
                 └───────┬───────┘
                     Yes │ No
                         │
             ┌───────────┘ └───────────┐
             ↓                         ↓
      ┌──────────────┐          ┌──────────────┐
      │ Protected App│          │ Login Screen │
      └──────────────┘          └──────┬───────┘
                                       ↓
                                    login()
                                       ↓
                                 router.replace("/")

25. Key Architectural Principles
Principle 1 — Use route groups to organize authentication boundaries
app/
├── login.tsx
└── (protected)/


The route group gives the protected portion of the application its own layout where authentication checks can occur.

Principle 2 — Use redirects instead of conditionally defining routes

Expo Router's file-based architecture means the routes exist regardless of authentication state.

Authentication determines where the user is allowed to go, not whether the route files exist.

Principle 3 — Protect the highest common layout

Put authentication checks in:

app/(protected)/_layout.tsx


rather than duplicating checks across every protected screen.

Principle 4 — Never treat the initial Boolean as the final authentication state

At startup:

false ≠ definitely logged out


It may simply mean:

authentication state hasn't loaded yet


Use:

isReady


to distinguish these states.

Principle 5 — Replace authentication routes

Use:

router.replace()


for login/logout transitions so authentication screens don't remain in the navigation history.

Principle 6 — Keep the splash screen up during initialization

The user should see:

Splash → Correct initial screen


rather than:

Splash → Blank/Login → Correct initial screen

26. Definition of Done

The authentication flow can be considered production-ready from a routing/UX perspective when:

A new user opens the application and reaches the login screen.
A user successfully authenticates and enters the protected application.
The login screen is removed from navigation history.
The authenticated state survives an application restart.
An authenticated user opening the application goes directly to the protected application.
An unauthenticated user attempting to deep-link to a protected screen is redirected to login.
Authentication initialization does not cause visual flashes.
Logout removes access to protected routes.
Login/logout transitions do not display inappropriate navigation animations.
The authentication state can later be replaced with a real backend/token-based implementation without requiring a redesign of the route structure.