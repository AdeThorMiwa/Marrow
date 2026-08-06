import axios, { AxiosError } from 'axios';
import { Platform } from 'react-native';

const API_PORT = 8081;

// Android emulator can't reach the host machine via `localhost` — it maps
// `10.0.2.2` to the host loopback instead. iOS simulator and web both share
// the host's network namespace directly. A physical device needs the host's
// LAN IP, which isn't knowable here — override via EXPO_PUBLIC_API_BASE_URL.
function resolveBaseUrl(): string {
  const override = process.env.EXPO_PUBLIC_API_BASE_URL;
  if (override) return override;
  const host = Platform.OS === 'android' ? '10.0.2.2' : 'localhost';
  return `http://${host}:${API_PORT}`;
}

export const client = axios.create({
  baseURL: resolveBaseUrl(),
  headers: { 'Content-Type': 'application/json' },
});

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

client.interceptors.response.use(undefined, (error: AxiosError<{ error?: string }>) => {
  const status = error.response?.status ?? 0;
  const message = error.response?.data?.error ?? error.message;
  throw new ApiError(status, message);
});
