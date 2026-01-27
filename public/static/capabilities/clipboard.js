(function() {
function setClipboard(text) {
    if (!window.ws || window.ws.readyState !== WebSocket.OPEN) return;
    const encoder = new TextEncoder();
    const data = encoder.encode(text);
    
    // Packet structure:
    // [0] Type (1 byte)
    // [1-8] Sequence (8 bytes)
    // [9] Paste (1 byte)
    // [10-13] Length (4 bytes)
    // [14...] Content (N bytes)
    
    const packet = new Uint8Array(1 + 8 + 1 + 4 + data.length);
    const view = new DataView(packet.buffer);
    
    packet[0] = 9; // WS_TYPE_SET_CLIPBOARD
    
    // Sequence (8 bytes) - using 0 for now
    view.setBigUint64(1, 0n, false); // false for BigEndian (network byte order)
    
    // Paste (1 byte) - true/false
    packet[9] = 1; // paste = true
    
    // Length (4 bytes)
    view.setUint32(10, data.length, false); // false for BigEndian
    
    // Content
    packet.set(data, 14);
    
    // window.ws.send(packet);
    sendDataChannelMessage(window.dataChannelOrdered, packet);
    console.log("set clipboard to device:", text);
}

function getClipboard() {
    // if (!window.ws || window.ws.readyState !== WebSocket.OPEN) return;
    const packet = new Uint8Array(2);
    packet[0] = 8; // WS_TYPE_GET_CLIPBOARD
    packet[1] = 0; // COPY_KEY_NONE
    sendDataChannelMessage(window.dataChannelOrdered, packet);
    // window.ws.send(packet);
}

window.document.querySelector('#setClipboardButton').addEventListener('click', () => {
    navigator.clipboard.readText().then(text => setClipboard(text));
});

getClipboard()

})();