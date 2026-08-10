import { Modal, Pressable, View } from 'react-native';

import { useTheme } from '@/theme/theme-provider';

import { Button } from './button';
import { Surface } from './surface';
import { Text } from './text';

export type ConfirmDialogProps = {
  visible: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  confirmLoading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
};

export function ConfirmDialog({
  visible,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  confirmLoading,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const theme = useTheme();

  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onCancel}>
      <Pressable
        onPress={onCancel}
        style={{ flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', alignItems: 'center', justifyContent: 'center' }}>
        <Pressable onPress={(e) => e.stopPropagation()} style={{ width: '85%', maxWidth: 360 }}>
          <Surface style={{ gap: theme.spacing.sm }}>
            <Text variant="label">{title}</Text>
            <Text variant="body" tone="secondary">
              {message}
            </Text>
            <View style={{ flexDirection: 'row', gap: theme.spacing.sm, marginTop: theme.spacing.sm }}>
              <View style={{ flex: 1 }}>
                <Button variant="outline" onPress={onCancel} disabled={confirmLoading}>
                  {cancelLabel}
                </Button>
              </View>
              <View style={{ flex: 1 }}>
                <Button variant="ghost" onPress={onConfirm} disabled={confirmLoading}>
                  {confirmLoading ? 'Working…' : confirmLabel}
                </Button>
              </View>
            </View>
          </Surface>
        </Pressable>
      </Pressable>
    </Modal>
  );
}
