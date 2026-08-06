import { FontAwesome5, MaterialCommunityIcons } from '@expo/vector-icons';
import { View } from 'react-native';

import { useTheme } from '@/theme/theme-provider';

export type SourceLogoProps = { adapterId: string; size?: number };

// Static per-adapter icon (Twitter logo, Instagram logo, ...) — identifies
// which platform a piece of content came from, not the individual
// account's own avatar (which would need per-source data we don't fetch —
// see the "adapter logo, not account logo" clarification this was built
// against).
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

export function SourceLogo({ adapterId, size = 24 }: SourceLogoProps) {
  const theme = useTheme();
  const entry = ADAPTER_ICONS[adapterId];

  return (
    <View
      style={{
        width: size,
        height: size,
        borderWidth: theme.borderWidth,
        borderColor: theme.colors.ink,
        borderRadius: theme.radius,
        alignItems: 'center',
        justifyContent: 'center',
        overflow: 'hidden',
        backgroundColor: theme.colors.background,
      }}>
      {entry ? (
        // The icon set is chosen dynamically per adapter, so its `name` prop
        // union can't be statically narrowed here.
        <entry.Icon name={entry.name as any} size={size * 0.55} color={theme.colors.ink} />
      ) : null}
    </View>
  );
}
