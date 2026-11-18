import { effect, mount, signal, Text, View } from "nsgo";

function Clock() {
  const read = () => `${Date.now()} ${new Date().toISOString()}`;
  const now = signal(read());
  const fired = signal("waiting");
  effect(() => {
    const timer = setTimeout(() => fired.set(`fired at ${Date.now()}`), 100);
    return () => clearTimeout(timer);
  });
  return <View>
    <Text testID="now">{now}</Text>
    <Text testID="read" onPress={() => now.set(read())}>Read</Text>
    <Text testID="timer">{fired}</Text>
  </View>;
}

mount(Clock);
