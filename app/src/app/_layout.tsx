import '@/global.css';

import {
  SmoochSans_400Regular,
  SmoochSans_500Medium,
  SmoochSans_600SemiBold,
  SmoochSans_700Bold,
  useFonts,
} from '@expo-google-fonts/smooch-sans';
import * as SplashScreen from 'expo-splash-screen';
import { useEffect } from 'react';

import AppTabs from '@/components/app-tabs';
import { ThemeProvider } from '@/theme/theme-provider';

SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const [fontsLoaded] = useFonts({
    SmoochSans_400Regular,
    SmoochSans_500Medium,
    SmoochSans_600SemiBold,
    SmoochSans_700Bold,
  });

  useEffect(() => {
    if (fontsLoaded) SplashScreen.hideAsync();
  }, [fontsLoaded]);

  if (!fontsLoaded) return null;

  return (
    <ThemeProvider>
      <AppTabs />
    </ThemeProvider>
  );
}
