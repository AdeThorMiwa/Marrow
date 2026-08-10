import { client } from './api';
import type { CommentThread, ContentDetail } from './types';

export async function getContentDetail(id: string): Promise<ContentDetail> {
  const { data } = await client.get<ContentDetail>(`/contents/${id}`);
  return data;
}

export async function getComments(id: string, cursor?: string): Promise<CommentThread> {
  const { data } = await client.get<CommentThread>(`/contents/${id}/comments`, { params: { cursor } });
  return data;
}
