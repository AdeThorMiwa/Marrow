import { FontAwesome5, MaterialCommunityIcons } from '@expo/vector-icons';
import { useState } from 'react';
import { Image, View } from 'react-native';

import { useTheme } from '@/theme/theme-provider';

import { Text } from './text';

export type SourceLogoProps = {
  adapterId: string;
  size?: number;
  // When either is given, SourceLogo renders this specific source's own
  // identity (its real logo image, or initials from its name) instead of
  // the generic per-adapter platform icon — used by the source rail, where
  // each item IS one specific source. Feed cards pass adapterId only and
  // keep the platform-icon behavior, since ContentPayload doesn't carry a
  // per-source logo.
  logoUrl?: string;
  name?: string;
};

// Static per-adapter icon (Twitter logo, Instagram logo, ...) — used when
// no logoUrl/name is given (see SourceLogoProps).
const ADAPTER_ICONS: Record<
  string,
  { Icon: typeof FontAwesome5 | typeof MaterialCommunityIcons; name: string }
> = {
  twitter: { Icon: FontAwesome5, name: 'twitter' },
  instagram: { Icon: FontAwesome5, name: 'instagram' },
  youtube: { Icon: FontAwesome5, name: 'youtube' },
  substack: { Icon: MaterialCommunityIcons, name: 'email-newsletter' },
  'rss-media': { Icon: MaterialCommunityIcons, name: 'rss' },
};

// First letter of the first two words, capitalized — never more than 2
// letters, even for a single-word name.
function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return '?';
  return words
    .slice(0, 2)
    .map((w) => w.charAt(0).toUpperCase())
    .join('');
}

export function SourceLogo({ adapterId, size = 24, logoUrl, name }: SourceLogoProps) {
  const theme = useTheme();
  const [imageFailed, setImageFailed] = useState(false);
  const accountIdentity = logoUrl !== undefined || name !== undefined;
  const showImage = accountIdentity && !!logoUrl && !imageFailed;
  const entry = ADAPTER_ICONS[adapterId];

  return (
    <View
      style={{
        width: size,
        height: size,
        borderRadius: size / 2,
        borderWidth: theme.borderWidth,
        borderColor: theme.colors.ink,
        alignItems: 'center',
        justifyContent: 'center',
        overflow: 'hidden',
        backgroundColor: theme.colors.background,
      }}>
      {showImage ? (
        <Image
          source={{ uri: logoUrl }}
          style={{ width: size, height: size }}
          onError={() => setImageFailed(true)}
          accessibilityLabel={name}
        />
      ) : accountIdentity ? (
        <Text variant="label" accessibilityLabel={name}>
          {initials(name ?? '')}
        </Text>
      ) : entry ? (
        // The icon set is chosen dynamically per adapter, so its `name` prop
        // union can't be statically narrowed here.
        <entry.Icon name={entry.name as any} size={size * 0.55} color={theme.colors.ink} />
      ) : null}
    </View>
  );
}
