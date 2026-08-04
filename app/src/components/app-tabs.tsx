import { NativeTabs } from 'expo-router/unstable-native-tabs';

import { useTheme } from '@/theme/theme-provider';

// No icons — iconography set is explicitly deferred (design-system Out of
// Scope). Labels only.
export default function AppTabs() {
  const theme = useTheme();

  return (
    <NativeTabs
      backgroundColor={theme.colors.background}
      indicatorColor={theme.colors.background}
      tintColor={theme.colors.ink}
      labelStyle={{ selected: { color: theme.colors.ink } }}>
      <NativeTabs.Trigger name="index">
        <NativeTabs.Trigger.Label>Home</NativeTabs.Trigger.Label>
      </NativeTabs.Trigger>

      <NativeTabs.Trigger name="explore">
        <NativeTabs.Trigger.Label>Components</NativeTabs.Trigger.Label>
      </NativeTabs.Trigger>
    </NativeTabs>
  );
}
