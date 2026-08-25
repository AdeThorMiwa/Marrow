import { client } from './api';
import type { Group } from './types';

export async function listGroups(): Promise<Group[]> {
  const { data } = await client.get<Group[]>('/groups');
  return data;
}

export async function createGroup(name: string, icon: string): Promise<Group> {
  const { data } = await client.post<Group>('/groups', { name, icon });
  return data;
}

export async function addSourceToGroup(sourceId: string, groupId: string): Promise<void> {
  await client.post(`/sources/${sourceId}/groups`, { group_id: groupId });
}

export async function removeSourceFromGroup(sourceId: string, groupId: string): Promise<void> {
  await client.delete(`/sources/${sourceId}/groups/${groupId}`);
}

export async function pauseGroup(id: string): Promise<void> {
  await client.post(`/groups/${id}/pause`);
}

export async function unpauseGroup(id: string): Promise<void> {
  await client.post(`/groups/${id}/unpause`);
}
