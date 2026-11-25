import WebSR from 'https://esm.sh/@websr/websr';

let websr = null;
let outputCanvas = null;

self.onmessage = async (event) => {
    const { type } = event.data;

    if (type === 'init') {
        outputCanvas = event.data.canvas;
        try {
            const gpu = await WebSR.initWebGPU();
            if (!gpu) {
                throw new Error('This browser or device does not support WebGPU');
            }
            
            let resolution_width = event.data.resolution_width;
            let resolution_height = event.data.resolution_height;
            if (resolution_width < resolution_height) {
                const temp = resolution_width;
                resolution_width = resolution_height;
                resolution_height = temp;
            }
            // If use webSR Version cnn-s:
            // const network_name = 'websr/cnn-s';
            // const weightsUrl = new URL('./cnn-s.bin', import.meta.url);
            // const response = await fetch(weightsUrl);
            // if (!response.ok) {
            //     throw new Error(`Failed to load WebSR weights (${response.status})`);
            // }
            // const weights = WebSR.parseBinaryWeights(await response.arrayBuffer())
            // If use original Animate4K cnn models:
            // const network_name = "animate4k/cnn-2x-s";
            // const weights = await (await fetch('./cnn-2x-s-an.json')).json()
            let network_name, weights;
            if (isMobile()) {
                network_name = "animate4k/cnn-2x-s";
                weights = await (await fetch('./cnn-2x-s-an.json')).json();
            } else if (isLowerThan1080P(resolution_width, resolution_height)) {
                network_name = "websr/cnn-s";
                weights = await (await fetch('./cnn-s.bin')).arrayBuffer();
                weights = WebSR.parseBinaryWeights(weights);
            } else if (is1080P(resolution_width, resolution_height)) {
                network_name = "anime4k/cnn-2x-m";
                weights = await (await fetch('./cnn-2x-m-an.json')).json();
            } else if (isEqualHigherThan4K(resolution_width, resolution_height)) {
                network_name = "anime4k/cnn-restore-m";
                weights = await (await fetch('./cnn-restore-m-an.json')).json();
            }  else {
                network_name = "anime4k/cnn-2x-s";
                weights = await (await fetch('./cnn-2x-s-an.json')).json();
            }

            console.log(`Resolution: ${resolution_width}x${resolution_height}, using network: ${network_name}`);

            websr = new WebSR({
                network_name: network_name,
                weights: weights,
                gpu,
                canvas: outputCanvas
            });

            self.postMessage({ type: 'ready' });
        } catch (error) {
            self.postMessage({ type: 'error', message: error.message || String(error) });
        }
        return;
    }

    if (type === 'frame') {
        const frame = event.data.frame;
        try {
            if (!websr) {
                throw new Error('WebSR worker is not initialized');
            }
            await websr.render(frame);
            self.postMessage({
                type: 'done',
                width: outputCanvas.width,
                height: outputCanvas.height
            });
        } catch (error) {
            self.postMessage({ type: 'error', message: error.message || String(error) });
        } finally {
            frame.close();
        }
    }
};

function isMobile() {
    return /Mobi|Android/i.test(navigator.userAgent);
}

function isLowerThan1080P(width, height) {
    return width < 1920 || height < 1080;
}

function isEqualHigherThan4K(width, height) {
    return width >= 3840 || height >= 2160;
}

function is1080P(width, height) {
    return width === 1920 && height === 1080;
}