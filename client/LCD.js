import { StyleSheet, SafeAreaView, Pressable, Text, View, Button } from 'react-native';

export default function LCD({lcdState}) {
	const s = StyleSheet.create({
		lcd: {
			backgroundColor: 'rgb(203, 232, 7)',
			flex: 1,
			padding: 10,
		},
	});
	return (
		<View style={s.lcd}>
		<Text>Hi</Text>
		</View>
	)
}
