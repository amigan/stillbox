import { StyleSheet, SafeAreaView, Pressable, Text, View, Button } from 'react-native';

export default function ScannerButton({label}) {
	const styles = StyleSheet.create({
		bText: {
			textAlign: 'center',
			color: '#fff'
		},
	});
	function pressedStyle(state) {
		return [
			{
				padding: 10,
				backgroundColor: state.pressed ? '#aaa' : '#000',
				color: '#fff',
			},
	       ];
	}

	return (
	    <Pressable style={pressedStyle}>
	      <Text style={styles.bText}>{label}</Text>
	    </Pressable>
	)
}

