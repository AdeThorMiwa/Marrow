import {
  Tabs,
  TabList,
  TabTrigger,
  TabSlot,
  type TabTriggerSlotProps,
  type TabListProps,
} from 'expo-router/ui';
import { Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { useTheme } from '@/theme/theme-provider';

import { Text } from './ui/text';

// Built on the headless expo-router/ui primitives rather than
// unstable-native-tabs — that API reserves native OS icon-slot space even
// with no icon set, which is what made the bar tall and put the label
// outside the actual tappable area. This gives full control over both.
export default function AppTabs() {
  return (
    <Tabs>
      <TabSlot style={{ flex: 1 }} />
      <TabList asChild>
        <BoxyTabList>
          <TabTrigger name="home" href="/" asChild>
            <TabButton>Home</TabButton>
          </TabTrigger>
          <TabTrigger name="sources" href="/sources" asChild>
            <TabButton>Sources</TabButton>
          </TabTrigger>
          <TabTrigger name="explore" href="/explore" asChild>
            <TabButton>Components</TabButton>
          </TabTrigger>
        </BoxyTabList>
      </TabList>
    </Tabs>
  );
}

function TabButton({ children, isFocused, ...props }: TabTriggerSlotProps) {
  const theme = useTheme();

  return (
    <Pressable
      {...props}
      style={{
        flex: 1,
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: 44,
        paddingVertical: theme.spacing.sm,
      }}>
      <Text variant="label" tone={isFocused ? 'primary' : 'secondary'}>
        {children}
      </Text>
    </Pressable>
  );
}

function BoxyTabList({ children, ...props }: TabListProps) {
  const theme = useTheme();
  const insets = useSafeAreaInsets();

  return (
    <View
      {...props}
      style={{
        flexDirection: 'row',
        backgroundColor: theme.colors.background,
        paddingBottom: insets.bottom,
      }}>
      {children}
    </View>
  );
}
