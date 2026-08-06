import { client } from './api';
import type { Source, SourceConfig } from './types';

export async function listSources(): Promise<Source[]> {
  const { data } = await client.get<Source[]>('/sources');
  return data;
}

export async function resolveSource(identifier: string): Promise<SourceConfig[]> {
  const { data } = await client.post<{ candidates: SourceConfig[] }>('/sources/resolve', { identifier });
  return data.candidates;
}

export async function createSources(sources: SourceConfig[]): Promise<Source[]> {
  const { data } = await client.post<Source[]>('/sources', { sources });
  return data;
}
