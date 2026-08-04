import { View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Text } from '@/components/ui';
import { useTheme } from '@/theme/theme-provider';

export default function HomeScreen() {
  const theme = useTheme();

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.colors.background }}>
      <View
        style={{
          flex: 1,
          justifyContent: 'center',
          alignItems: 'center',
          gap: theme.spacing.sm,
          paddingHorizontal: theme.spacing.lg,
        }}>
        <Text variant="itemTitle">Marrow</Text>
        <Text variant="body" tone="secondary" style={{ textAlign: 'center' }}>
          Consume what matters. Retain what counts.
        </Text>
      </View>
    </SafeAreaView>
  );
}
