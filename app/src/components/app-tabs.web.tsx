import {
  Tabs,
  TabList,
  TabTrigger,
  TabSlot,
  type TabTriggerSlotProps,
  type TabListProps,
} from 'expo-router/ui';
import { Pressable, View } from 'react-native';

import { useTheme } from '@/theme/theme-provider';

import { Text } from './ui/text';

export default function AppTabs() {
  return (
    <Tabs>
      <TabSlot style={{ flex: 1 }} />
      <TabList asChild>
        <BoxyTabList>
          <TabTrigger name="home" href="/" asChild>
            <TabButton>Home</TabButton>
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
        paddingVertical: theme.spacing.sm,
        paddingHorizontal: theme.spacing.lg,
        borderBottomWidth: isFocused ? theme.borderWidthError : 0,
        borderColor: theme.colors.ink,
      }}>
      <Text variant="label" tone={isFocused ? 'primary' : 'secondary'}>
        {children}
      </Text>
    </Pressable>
  );
}

function BoxyTabList({ children, ...props }: TabListProps) {
  const theme = useTheme();

  return (
    <View
      {...props}
      style={{
        flexDirection: 'row',
        justifyContent: 'center',
        borderTopWidth: theme.borderWidth,
        borderColor: theme.colors.ink,
        backgroundColor: theme.colors.background,
      }}>
      {children}
    </View>
  );
}
