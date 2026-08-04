import { ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Avatar, Badge, Button, Surface, Text, TextInput } from '@/components/ui';
import { useTheme } from '@/theme/theme-provider';

// Living component gallery — not a product screen. Exercises every
// primitive for visual/cross-platform verification (design-system tasks.md #13).
export default function ComponentsScreen() {
  const theme = useTheme();

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.colors.background }}>
      <ScrollView contentContainerStyle={{ padding: theme.spacing.lg, gap: theme.spacing.xl }}>
        <Section title="Typography">
          <Text variant="itemTitle">Item title (serif)</Text>
          <Text variant="body">Body text (sans)</Text>
          <Text variant="body" tone="secondary">
            Secondary tone
          </Text>
          {/* inkTertiary is AA-Large only (~4.47:1) — correct usage stays at
              >=18pt/14pt-bold. `caption` (12pt/400) would NOT qualify. */}
          <Text variant="bodyLarge" tone="tertiary">
            Tertiary text — restricted to larger/bold sizes to clear AA-Large
          </Text>
          <Text variant="body" voice="ai">
            AI-authored text renders italic — the only non-color distinction available.
          </Text>
        </Section>

        <Section title="Buttons">
          <View style={{ flexDirection: 'row', gap: theme.spacing.sm, flexWrap: 'wrap' }}>
            <Button variant="solid" onPress={() => {}}>
              Solid
            </Button>
            <Button variant="outline" onPress={() => {}}>
              Outline
            </Button>
            <Button variant="ghost" onPress={() => {}}>
              Ghost
            </Button>
            <Button variant="solid" disabled onPress={() => {}}>
              Disabled
            </Button>
          </View>
        </Section>

        <Section title="Badges">
          <View style={{ flexDirection: 'row', gap: theme.spacing.sm }}>
            <Badge variant="outline">STALE</Badge>
            <Badge variant="solid">BROKEN</Badge>
          </View>
        </Section>

        <Section title="Surface">
          <Surface>
            <Text variant="body">A bordered surface — no shadow, no radius.</Text>
          </Surface>
        </Section>

        <Section title="Text Input">
          <TextInput label="Source URL" placeholder="https://example.com" />
          <TextInput label="With error" error="Could not resolve this URL." />
        </Section>

        <Section title="Avatar (square, not circular)">
          <View style={{ flexDirection: 'row', gap: theme.spacing.sm, alignItems: 'center' }}>
            <Avatar name="Jane Doe" size="sm" />
            <Avatar name="Marrow" size="md" />
            <Avatar name="A" size="lg" />
          </View>
        </Section>
      </ScrollView>
    </SafeAreaView>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  const theme = useTheme();
  return (
    <View style={{ gap: theme.spacing.sm }}>
      <Text variant="heading3">{title}</Text>
      {children}
    </View>
  );
}
