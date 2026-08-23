import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useState } from 'react';
import { Modal, Pressable, View } from 'react-native';

import { useTheme } from '@/theme/theme-provider';

import { Button } from './button';
import { Surface } from './surface';
import { Text } from './text';
import { TextInput } from './text-input';

export type CreateGroupDialogProps = {
  visible: boolean;
  loading?: boolean;
  onCreate: (name: string, icon: string) => void;
  onCancel: () => void;
};

// Small curated set — icon is just a plain MaterialCommunityIcons glyph
// name (see docs/source-groups/design.md §2), not a full icon browser.
const ICON_CHOICES = [
  'folder',
  'star',
  'newspaper-variant',
  'video',
  'microphone',
  'book-open-variant',
  'briefcase',
  'flask',
  'code-tags',
  'palette',
  'earth',
  'rocket-launch',
];

export function CreateGroupDialog({ visible, loading, onCreate, onCancel }: CreateGroupDialogProps) {
  const theme = useTheme();
  const [name, setName] = useState('');
  const [icon, setIcon] = useState(ICON_CHOICES[0]);

  const reset = () => {
    setName('');
    setIcon(ICON_CHOICES[0]);
  };

  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onCancel}>
      <Pressable
        onPress={onCancel}
        style={{ flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', alignItems: 'center', justifyContent: 'center' }}>
        <Pressable onPress={(e) => e.stopPropagation()} style={{ width: '85%', maxWidth: 360 }}>
          <Surface style={{ gap: theme.spacing.md }}>
            <Text variant="label">New group</Text>

            <TextInput label="Name" value={name} onChangeText={setName} autoFocus />

            <View style={{ gap: theme.spacing.xs }}>
              <Text variant="label" tone="secondary">
                Icon
              </Text>
              <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: theme.spacing.sm }}>
                {ICON_CHOICES.map((choice) => {
                  const selected = choice === icon;
                  return (
                    <Pressable
                      key={choice}
                      onPress={() => setIcon(choice)}
                      style={{
                        width: 44,
                        height: 44,
                        borderRadius: 22,
                        alignItems: 'center',
                        justifyContent: 'center',
                        backgroundColor: theme.colors.background,
                        borderWidth: selected ? theme.borderWidthError : theme.borderWidth,
                        borderColor: theme.colors.ink,
                      }}>
                      <MaterialCommunityIcons name={choice as any} size={22} color={theme.colors.ink} />
                    </Pressable>
                  );
                })}
              </View>
            </View>

            <View style={{ flexDirection: 'row', gap: theme.spacing.sm, marginTop: theme.spacing.sm }}>
              <View style={{ flex: 1 }}>
                <Button
                  variant="outline"
                  onPress={() => {
                    reset();
                    onCancel();
                  }}
                  disabled={loading}>
                  Cancel
                </Button>
              </View>
              <View style={{ flex: 1 }}>
                <Button
                  variant="ghost"
                  onPress={() => {
                    if (!name.trim()) return;
                    onCreate(name.trim(), icon);
                    reset();
                  }}
                  disabled={loading || !name.trim()}>
                  {loading ? 'Creating…' : 'Create'}
                </Button>
              </View>
            </View>
          </Surface>
        </Pressable>
      </Pressable>
    </Modal>
  );
}
