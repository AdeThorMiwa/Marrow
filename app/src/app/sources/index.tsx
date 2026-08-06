import { useCallback, useEffect, useState } from 'react';
import { FlatList, RefreshControl, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { router } from 'expo-router';

import { Badge, Button, Surface, Text } from '@/components/ui';
import { ApiError } from '@/lib/api';
import { listSources } from '@/lib/source';
import type { Source, SourceHealth } from '@/lib/types';
import { useTheme } from '@/theme/theme-provider';

const HEALTH_LABEL: Record<SourceHealth, string> = {
  ok: 'OK',
  stale: 'STALE',
  broken: 'BROKEN',
};

export default function SourcesScreen() {
  const theme = useTheme();
  const [sources, setSources] = useState<Source[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const load = useCallback(async () => {
    try {
      setError(null);
      const data = await listSources();
      setSources(data);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to load sources.');
    }
  }, []);

  useEffect(() => {
    let ignore = false;
    (async () => {
      setError(null);
      try {
        const data = await listSources();
        if (!ignore) setSources(data);
      } catch (e) {
        if (!ignore) setError(e instanceof ApiError ? e.message : 'Failed to load sources.');
      }
    })();
    return () => {
      ignore = true;
    };
  }, []);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await load();
    setRefreshing(false);
  }, [load]);

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.colors.background }}>
      <View
        style={{
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'space-between',
          paddingHorizontal: theme.spacing.lg,
          paddingVertical: theme.spacing.md,
        }}>
        <Text variant="heading2">Sources</Text>
        <Button variant='ghost' size="sm" onPress={() => router.push('/sources/add')}>
          Add source
        </Button>
      </View>

      {error ? (
        <View style={{ padding: theme.spacing.lg, gap: theme.spacing.sm }}>
          <Text variant="body" tone="secondary">
            {error}
          </Text>
          <Button variant="outline" size="sm" onPress={load}>
            Retry
          </Button>
        </View>
      ) : null}

      {sources && sources.length === 0 ? (
        <View style={{ padding: theme.spacing.lg }}>
          <Text variant="body" tone="secondary">
            No sources yet. Add one to start pulling in content.
          </Text>
        </View>
      ) : null}

      <FlatList
        data={sources ?? []}
        keyExtractor={(item) => item.id}
        contentContainerStyle={{ padding: theme.spacing.lg, gap: theme.spacing.sm }}
        refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} />}
        renderItem={({ item }) => <SourceRow source={item} />}
      />
    </SafeAreaView>
  );
}

function SourceRow({ source }: { source: Source }) {
  const theme = useTheme();
  return (
    <Surface style={{ gap: theme.spacing.xs }}>
      <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Text variant="itemTitle" style={{ flex: 1 }}>
          {source.name}
        </Text>
        <Badge variant={source.health === 'ok' ? 'outline' : 'solid'}>{HEALTH_LABEL[source.health]}</Badge>
      </View>
      <Text variant="body" tone="secondary">
        {source.identifier}
      </Text>
      <Text variant="caption" tone="secondary">
        {source.adapter_id}
        {source.consecutive_failures > 0 ? ` · ${source.consecutive_failures} consecutive failures` : ''}
      </Text>
    </Surface>
  );
}
