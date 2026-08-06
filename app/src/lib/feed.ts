import { client } from './api';
import type { FeedPage } from './types';

export async function getFeed(cursor?: string, limit?: number): Promise<FeedPage> {
  const { data } = await client.get<FeedPage>('/feed', { params: { cursor, limit } });
  return data;
}
