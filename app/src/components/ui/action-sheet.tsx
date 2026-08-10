import { Modal, Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { useTheme } from '@/theme/theme-provider';

import { Text } from './text';
import { Surface } from './surface';
import { Button } from './button';

export type ActionSheetAction = {
  label: string;
  onPress: () => void;
  destructive?: boolean;
};

export type ActionSheetProps = {
  visible: boolean;
  onClose: () => void;
  actions: ActionSheetAction[];
};

// Fixed black regardless of the active theme — an action sheet reads as a
// distinct system-level surface (iOS/Android's own share/action sheets
// both do this), not just another themed card floating over the page.
const SHEET_BACKGROUND = '#000000';
const SHEET_DIVIDER = '#262626';
const SHEET_TEXT = '#FFFFFF';

// A true bottom sheet: flush to the screen's edges and bottom, rounded top
// corners only, solid black — not a floating, theme-colored card, which is
// what made the previous version read as "a stack of buttons over a dimmed
// page" instead of an actual sheet.
export function ActionSheet({ visible, onClose, actions }: ActionSheetProps) {
  const theme = useTheme();
  const insets = useSafeAreaInsets();

  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <Pressable
        onPress={onClose}
        style={{ flex: 1, backgroundColor: 'rgba(0,0,0,0.6)', justifyContent: 'flex-end' }}>
        <View className='flex flex-col bg-black w-full border border-t-[#262626] rounded-t-[20px]'>
          {actions.map((action, i) => (
            <Button className='py-8' key={i} variant='ghost' onPress={() => {
              onClose();
              action.onPress();
            }}>{action.label}</Button>
          ))}
            <View style={{ height: theme.hairlineWidth, backgroundColor: SHEET_DIVIDER }} />
            <Button className='py-8' variant='ghost' onPress={onClose}>Close</Button>
        </View>

      </Pressable>
    </Modal>
  );
}
