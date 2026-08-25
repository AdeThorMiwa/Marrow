import { client } from './api';
import type { FeedPage } from './types';

export type FeedFilter = { sourceIds: string[]; groupIds: string[] };

export async function getFeed(cursor?: string, limit?: number, filter?: FeedFilter): Promise<FeedPage> {
  const { data } = await client.get<FeedPage>('/feed', {
    params: {
      cursor,
      limit,
      source_ids: filter?.sourceIds.length ? filter.sourceIds.join(',') : undefined,
      group_ids: filter?.groupIds.length ? filter.groupIds.join(',') : undefined,
    },
  });
  return data;
}
