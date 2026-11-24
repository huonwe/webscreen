let videoDecoder = null;
let audioDecoder = null;
let canvasContext = null;
let videoConfig = null;
let audioConfig = null;
let videoNeedsKeyFrame = true;
let lastVideoTimestamp = -1;
let lastAudioTimestamp = -1;
let upscaleEnabled = false;

async function initialize({ canvas, codec, width, height, upscalingEnabled }) {
    upscaleEnabled = Boolean(upscalingEnabled);
    if (!self.VideoDecoder || !self.EncodedVideoChunk || !self.AudioDecoder || !self.EncodedAudioChunk) {
        throw new Error('This worker does not support WebCodecs audio/video decoding');
    }

    switch (codec.toLowerCase()) {
        case 'h264':
            videoConfig = { codec: 'avc3.640028', codedWidth: width, codedHeight: height };
            break;
        case 'h265':
        case 'hevc':
            videoConfig = { codec: 'hvc1.1.6.L93.B0', codedWidth: width, codedHeight: height };
            break;
        case 'av1':
            videoConfig = { codec: 'av01.0.08M.08', codedWidth: width, codedHeight: height };
            break;
        default:
            throw new Error(`Unsupported video codec: ${codec}`);
    }

    const videoSupport = await VideoDecoder.isConfigSupported(videoConfig);
    if (!videoSupport.supported) {
        throw new Error(`Worker cannot decode ${videoConfig.codec}`);
    }
    audioConfig = { codec: 'opus', sampleRate: 48000, numberOfChannels: 2 };
    const audioSupport = await AudioDecoder.isConfigSupported(audioConfig);
    if (!audioSupport.supported) {
        self.postMessage({ type: 'warning', message: 'Worker cannot decode Opus audio' });
    }

    canvas.width = width;
    canvas.height = height;
    canvasContext = canvas.getContext('2d');

    videoDecoder = new VideoDecoder({
        output(frame) {
            if (upscaleEnabled) {
                try {
                    if (canvas.width !== frame.displayWidth) canvas.width = frame.displayWidth;
                    if (canvas.height !== frame.displayHeight) canvas.height = frame.displayHeight;
                    self.postMessage({ type: 'source-frame', frame }, [frame]);
                } catch (error) {
                    frame.close();
                    reportError(`Failed to transfer decoded video: ${error.message}`);
                }
                return;
            }
            try {
                if (canvas.width !== frame.displayWidth) canvas.width = frame.displayWidth;
                if (canvas.height !== frame.displayHeight) canvas.height = frame.displayHeight;
                canvasContext.drawImage(frame, 0, 0, canvas.width, canvas.height);
            } catch (error) {
                reportError(`Failed to render decoded video: ${error.message}`);
            } finally {
                frame.close();
            }
        },
        error(error) {
            reportError(`VideoDecoder error: ${error.message}`);
            videoNeedsKeyFrame = true;
            try {
                videoDecoder.reset();
                videoDecoder.configure(videoSupport.config);
            } catch (resetError) {
                reportError(`Failed to reset VideoDecoder: ${resetError.message}`);
            }
        }
    });
    videoDecoder.configure(videoSupport.config);

    if (audioSupport.supported) {
        audioDecoder = new AudioDecoder({
            output(audioData) {
                try {
                    self.postMessage({ type: 'audio', audioData }, [audioData]);
                } catch (error) {
                    audioData.close();
                    reportError(`Failed to transfer decoded audio: ${error.message}`);
                }
            },
            error(error) {
                reportError(`AudioDecoder error: ${error.message}`);
                lastAudioTimestamp = -1;
                try {
                    audioDecoder.reset();
                    audioDecoder.configure(audioSupport.config);
                } catch (resetError) {
                    reportError(`Failed to reset AudioDecoder: ${resetError.message}`);
                }
            }
        });
        audioDecoder.configure(audioSupport.config);
    }
}

function decodeFrame(buffer) {
    if (buffer.byteLength < 10) return;

    const bytes = new Uint8Array(buffer);
    const kind = bytes[0];
    const keyFrame = bytes[1] === 1;
    const pts = Number(new DataView(buffer).getBigUint64(2, false));
    const payload = bytes.subarray(10);

    if (kind === 0) {
        if (!videoDecoder || videoDecoder.state !== 'configured') return;
        if (videoDecoder.decodeQueueSize > 8) {
            videoNeedsKeyFrame = true;
            return;
        }
        if (videoNeedsKeyFrame && !keyFrame) return;
        if (keyFrame) videoNeedsKeyFrame = false;

        const timestamp = pts > lastVideoTimestamp ? pts : lastVideoTimestamp + 16667;
        lastVideoTimestamp = timestamp;
        videoDecoder.decode(new EncodedVideoChunk({
            type: keyFrame ? 'key' : 'delta',
            timestamp,
            data: payload
        }));
        return;
    }

    if (kind === 1) {
        if (!audioDecoder || audioDecoder.state !== 'configured' || payload.length === 0) return;
        if (audioDecoder.decodeQueueSize > 8) {
            lastAudioTimestamp = -1;
            return;
        }

        const timestamp = pts > lastAudioTimestamp ? pts : lastAudioTimestamp + 20000;
        lastAudioTimestamp = timestamp;
        audioDecoder.decode(new EncodedAudioChunk({
            type: 'key',
            timestamp,
            data: payload
        }));
    }
}

function reportError(message) {
    self.postMessage({ type: 'error', message });
}

self.onmessage = async (event) => {
    const { type } = event.data;
    try {
        if (type === 'init') {
            await initialize(event.data);
            self.postMessage({ type: 'ready', audioAvailable: Boolean(audioDecoder) });
        } else if (type === 'frame') {
            decodeFrame(event.data.buffer);
        } else if (type === 'upscaling') {
            upscaleEnabled = Boolean(event.data.enabled);
        } else if (type === 'close') {
            videoDecoder?.close();
            audioDecoder?.close();
            self.close();
        }
    } catch (error) {
        reportError(error.message || String(error));
    }
};
