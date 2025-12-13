const videoElement = document.getElementById('remoteVideo');

function getScreenCoordinates(event) {
    // Ensure video metadata is loaded
    if (!videoElement.videoWidth || !videoElement.videoHeight) {
        return null;
    }

    // Calculate scale factors
    // Since we use max-width/max-height and display:block, the video element
    // size exactly matches the rendered video size.
    const scaleX = videoElement.videoWidth / videoElement.clientWidth;
    const scaleY = videoElement.videoHeight / videoElement.clientHeight;

    // Calculate coordinates
    let clientX, clientY;

    if (event.touches && event.touches.length > 0) {
        clientX = event.touches[0].clientX;
        clientY = event.touches[0].clientY;
    } else if (event.changedTouches && event.changedTouches.length > 0) {
        clientX = event.changedTouches[0].clientX;
        clientY = event.changedTouches[0].clientY;
    } else {
        clientX = event.clientX;
        clientY = event.clientY;
    }

    const rect = videoElement.getBoundingClientRect();
    const offsetX = clientX - rect.left;
    const offsetY = clientY - rect.top;

    const x = Math.round(offsetX * scaleX);
    const y = Math.round(offsetY * scaleY);

    // Clamp coordinates to be within video bounds (just in case)
    const clampedX = Math.max(0, Math.min(x, videoElement.videoWidth));
    const clampedY = Math.max(0, Math.min(y, videoElement.videoHeight));

    return { x: clampedX, y: clampedY };
}

videoElement.addEventListener('mousedown', (event) => {
    const coords = getScreenCoordinates(event);
    if (coords) {
        // console.log(`MouseDown at: ${coords.x}, ${coords.y}`);
        p = createTouchPacket(TOUCH_ACTION_DOWN, 0, coords.x, coords.y);
        sendWSMessage(p);
    }
});

videoElement.addEventListener('mouseup', (event) => {
    const coords = getScreenCoordinates(event);
    if (coords) {
        // console.log(`MouseUp at: ${coords.x}, ${coords.y}`);
        p = createTouchPacket(TOUCH_ACTION_UP, 0, coords.x, coords.y);
        sendWSMessage(p);
    }
});
videoElement.addEventListener('mousemove', (event) => {
    if (event.buttons !== 1) return; // Only when left button is pressed
    const coords = getScreenCoordinates(event);
    if (coords) {
        // console.log(`MouseMove at: ${coords.x}, ${coords.y}`);
        p = createTouchPacket(TOUCH_ACTION_MOVE, 0, coords.x, coords.y);
        sendWSMessage(p);
    }
});

videoElement.addEventListener('touchstart', (event) => {
    event.preventDefault(); // Prevent scrolling/mouse emulation
    const coords = getScreenCoordinates(event);
    if (coords) {
        // console.log(`TouchStart at: ${coords.x}, ${coords.y}`);
        p = createTouchPacket(TOUCH_ACTION_DOWN, 0, coords.x, coords.y);
        sendWSMessage(p);
    }
}, { passive: false });

videoElement.addEventListener('touchend', (event) => {
    event.preventDefault();
    const coords = getScreenCoordinates(event);
    if (coords) {
        // console.log(`TouchEnd at: ${coords.x}, ${coords.y}`);
        p = createTouchPacket(TOUCH_ACTION_UP, 0, coords.x, coords.y);
        sendWSMessage(p);
    }
}, { passive: false });

videoElement.addEventListener('touchmove', (event) => {
    event.preventDefault();
    const coords = getScreenCoordinates(event);
    if (coords) {
        // console.log(`TouchMove at: ${coords.x}, ${coords.y}`);
        p = createTouchPacket(TOUCH_ACTION_MOVE, 0, coords.x, coords.y);
        sendWSMessage(p);
    }
}, { passive: false });

function checkOrientation() {
    const video = document.getElementById('remoteVideo');
    if (!video.videoWidth || !video.videoHeight) return;

    const isPagePortrait = window.innerHeight > window.innerWidth;
    const isVideoLandscape = video.videoWidth > video.videoHeight;

    const isPageLandscape = window.innerWidth > window.innerHeight;
    const isVideoPortrait = video.videoHeight > video.videoWidth;

    if (isPagePortrait && isVideoLandscape) {
            // console.log("Auto-rotating: Page Portrait, Video Landscape");
            p = createRotatePacket();
            sendWSMessage(p);
        } else if (isPageLandscape && isVideoPortrait) {
            // console.log("Auto-rotating: Page Landscape, Video Portrait");
            p = createRotatePacket();
            sendWSMessage(p);
        }
}