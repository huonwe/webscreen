const TYPE_ROTATE    = 0x0B; // rotate event

// 这里WebRTC会自动通过RTCP请求关键帧，但我们也可以手动请求
const TYPE_RKF   = 0x63; // request key frame


function createRotatePacket() {
    const buffer = new ArrayBuffer(1);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_ROTATE);
    return buffer;
}

function createRequestKeyFramePacket() {
    const buffer = new ArrayBuffer(2);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_RKF);
    view.setUint8(1, 0);

    return buffer;
}

