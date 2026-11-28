import { showToast } from './screen.js';
import { getSetting } from './global-settings.js';

const params = new URLSearchParams(window.location.search);
const sessionConfig = Object.fromEntries(params.entries());

document.title = sessionConfig.device_id || CONFIG.device_uuid;

let webRTCWorker, webSocketWorker, webSRWorker;
let currentProtocol = sessionConfig.protocol_transport;
let wsConnection, webRTCConnection;
export function SendPacket(packet, ordered = true) {

}

export function GetSessionConfigValue(key) {

}

// export function CanControl() {

// }

export async function StartSession() {
    console.log(sessionConfig);
    if (!CONFIG.device_uuid) {
        console.error("No device configuration found. Please set up the device configuration in the console.");
        showToast(i18n.t('error_no_device_config'), 3000);
        return;
    }
    const handshakeWSPath = '/screen/ws';
    const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}${handshakeWSPath}`;
    const wsConnection = new WebSocket(wsUrl);
    wsConnection.onopen = () => {
        console.log("WebSocket connection established.");
        const extra = {};
        if (webRTCConnection) extra.sdp = webRTCConnection.localDescription.sdp;
        if (webRTCConnection && sessionConfig.av_sync) extra.av_sync = sessionConfig.av_sync;
        const connectionConfig = {
            protocol_transport: CONFIG.protocol_transport,
            video_codec: sessionConfig.video_codec || getSetting('video_codec'),
            audio_codec: sessionConfig.audio_codec || getSetting('audio_codec'),
            remote_uuid: sessionConfig.device_uuid,
            extra
        };
        wsConnection.send(JSON.stringify(connectionConfig));
        delete sessionConfig.protocol_transport;
        delete sessionConfig.device_uuid;
        delete sessionConfig.extra;
        delete sessionConfig.dont_trust_remote_timestamp;
        console.log("remote config:", connectionConfig);
        wsConnection.send(JSON.stringify(connectionConfig));
        wsConnection.onmessage = handleWebSocketMessage;
    }
}

function handleWebSocketMessage(event) {
    if (event instanceof MessageEvent) {
        const data = JSON.parse(event.data);
        if (data instanceof Object) {
            console.log("Received WebSocket message:", data);
        }
    }
    if (event instanceof ArrayBuffer || event instanceof Blob) {
        
    }
}