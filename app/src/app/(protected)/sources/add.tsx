import { useState } from 'react';
import { Pressable, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { router } from 'expo-router';

import { Button, Surface, Text, TextInput } from '@/components/ui';
import { ApiError } from '@/lib/api';
import { createSources, resolveSource } from '@/lib/source';
import type { SourceConfig } from '@/lib/types';
import { useTheme } from '@/theme/theme-provider';

export default function AddSourceScreen() {
  const theme = useTheme();
  const [identifier, setIdentifier] = useState('');
  const [candidates, setCandidates] = useState<SourceConfig[] | null>(null);
  const [selected, setSelected] = useState<SourceConfig | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [resolving, setResolving] = useState(false);
  const [adding, setAdding] = useState(false);

  const canResolve = identifier.trim().length > 0 && !resolving;

  const onResolve = async () => {
    if (!canResolve) return;
    setResolving(true);
    setError(null);
    setCandidates(null);
    setSelected(null);
    try {
      const found = await resolveSource(identifier.trim());
      if (found.length === 0) {
        setError('No publication found for this link.');
      } else {
        setCandidates(found);
        if (found.length === 1) setSelected(found[0]);
      }
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to resolve this link.');
    } finally {
      setResolving(false);
    }
  };

  const onAdd = async () => {
    if (!selected || adding) return;
    setAdding(true);
    setError(null);
    try {
      await createSources([selected]);
      router.back();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to add source.');
    } finally {
      setAdding(false);
    }
  };

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
        <Text variant="heading2">Add source</Text>
        <Button variant="ghost" size="sm" onPress={() => router.back()}>
          Cancel
        </Button>
      </View>

      <View style={{ padding: theme.spacing.lg, gap: theme.spacing.lg }}>
        <Text variant="body" tone="secondary">
          Paste a feed URL or a share link — a post, Note, profile, or podcast feed.
        </Text>

        <TextInput
          label="Source URL"
          placeholder="https://example.substack.com"
          value={identifier}
          onChangeText={(text) => {
            setIdentifier(text);
            setCandidates(null);
            setSelected(null);
          }}
          error={candidates === null ? (error ?? undefined) : undefined}
          autoCapitalize="none"
          autoCorrect={false}
          keyboardType="url"
          onSubmitEditing={onResolve}
        />

        {candidates === null ? (
          <Button variant='outline' onPress={onResolve} disabled={!canResolve}>
            {resolving ? 'Resolving…' : 'Resolve'}
          </Button>
        ) : (
          <View style={{ gap: theme.spacing.md }}>
            {candidates.length > 1 ? (
              <Text variant="body" tone="secondary">
                This link doesn&apos;t belong to one publication — pick which one to add.
              </Text>
            ) : null}

            <View style={{ gap: theme.spacing.sm }}>
              {candidates.map((c) => {
                const isSelected = selected?.identifier === c.identifier;
                return (
                  <Pressable key={c.identifier} onPress={() => setSelected(c)}>
                    <Surface style={{ borderWidth: isSelected ? theme.borderWidthError : theme.hairlineWidth }}>
                      <Text variant="itemTitle">{c.name}</Text>
                      <Text variant="caption" tone="secondary">
                        {c.identifier}
                      </Text>
                    </Surface>
                  </Pressable>
                );
              })}
            </View>

            {error ? (
              <Text variant="body" tone="secondary">
                {error}
              </Text>
            ) : null}

            <Button variant='outline' onPress={onAdd} disabled={!selected || adding}>
              {adding ? 'Adding…' : 'Add source'}
            </Button>
          </View>
        )}
      </View>
    </SafeAreaView>
  );
}
