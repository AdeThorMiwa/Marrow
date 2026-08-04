import { useState } from 'react';
import { Image, View } from 'react-native';

import { useTheme } from '@/theme/theme-provider';

import { Text } from './text';

export type AvatarProps = {
  name: string;
  imageUrl?: string;
  size?: 'sm' | 'md' | 'lg';
};

const DIMENSIONS = { sm: 24, md: 40, lg: 64 } as const;

function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  if (parts.length === 1) return parts[0]!.charAt(0).toUpperCase();
  return (parts[0]!.charAt(0) + parts[parts.length - 1]!.charAt(0)).toUpperCase();
}

// Square, not circular — Req 3.3's "radius zero, no exceptions" applies here
// too; the literal reading of "boxy" (Design §2.3).
export function Avatar({ name, imageUrl, size = 'md' }: AvatarProps) {
  const theme = useTheme();
  const [failed, setFailed] = useState(false);
  const dimension = DIMENSIONS[size];
  const showImage = !!imageUrl && !failed;

  return (
    <View
      style={{
        width: dimension,
        height: dimension,
        borderWidth: theme.borderWidth,
        borderColor: theme.colors.ink,
        borderRadius: theme.radius,
        alignItems: 'center',
        justifyContent: 'center',
        overflow: 'hidden',
        backgroundColor: theme.colors.background,
      }}>
      {showImage ? (
        <Image
          source={{ uri: imageUrl }}
          style={{ width: dimension, height: dimension }}
          onError={() => setFailed(true)}
          accessibilityLabel={name}
        />
      ) : (
        <Text variant={size === 'lg' ? 'heading3' : 'label'} accessibilityLabel={name}>
          {getInitials(name)}
        </Text>
      )}
    </View>
  );
}
