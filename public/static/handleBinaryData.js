import { ShowToast } from './screen.js';
const decoder = new TextDecoder();

function handleBinaryData(data) {
    const view = new Uint8Array(data);
    switch (view[0]) {
        // Video Frame
        case 0x00: {

        }
        // Audio Frame
        case 0x01: {

        }
        case 0x17: {
            const text = decoder.decode(view.slice(1));
            console.log("Clipboard from device:", text);
            try {
                navigator.clipboard.writeText(text).catch(err => {
                    console.error('Failed to write to clipboard:', err);
                });
            } catch (error) {
                console.error('Clipboard API not available:', error);
            }
            return;
        }
        case 0x64: {
            const textMsg = decoder.decode(view.slice(1));
            console.log("Text message from agent:", textMsg);
            ShowToast(textMsg, 3000);
            return;
        }
        case 0x65: {
            const width = (view[1] << 24) | (view[2] << 16) | (view[3] << 8) | view[4];
            const height = (view[5] << 24) | (view[6] << 16) | (view[7] << 8) | view[8];
            console.log("Video meta updated from agent: Width=" + width + ", Height=" + height);
            // window.videoResolution = { width, height };
            return;
        }
    }
}