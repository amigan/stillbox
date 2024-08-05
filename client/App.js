import { StyleSheet, SafeAreaView, Pressable, Text, View, Button } from 'react-native';
import ScannerButton from './Buttons.js';
import LCD from './LCD.js';


export default function App() {
  return (
	  <View style={{flex: 1, padding: 5, borderRadius: 55, overflow: 'hidden' }}>
	  <LCD />
	  <View style={styles.container}>
	    <View style={styles.button}>
	      <Button color="#FD5C6B" title="Scan" />
	      <Button color="#FF9233" title="Manual" />
	    </View>
	    <View style={styles.button}>
	      <ScannerButton label="SCAN" />
	      <ScannerButton label="MANUAL" />
	    </View>
	  </View>
	  </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    flexDirection: 'row',
	  flexWrap: 'wrap',
	  backgroundColor: '#000',
    alignItems: 'center',
    justifyContent: 'center',
  },
button: {
	width: 80,
},
	bText: {
		textAlign: 'center',
		color: '#fff'
	},
smButton: {
	margin: 5,
},
});
