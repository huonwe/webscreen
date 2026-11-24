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

            const weightsUrl = new URL('./cnn-s.bin', import.meta.url);
            const response = await fetch(weightsUrl);
            if (!response.ok) {
                throw new Error(`Failed to load WebSR weights (${response.status})`);
            }
            const weights = await response.arrayBuffer();

            websr = new WebSR({
                network_name: 'websr/cnn-s',
                weights: WebSR.parseBinaryWeights(weights),
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
        const bitmap = event.data.frame;
        try {
            if (!websr) {
                throw new Error('WebSR worker is not initialized');
            }
            await websr.render(bitmap);
            self.postMessage({
                type: 'done',
                width: outputCanvas.width,
                height: outputCanvas.height
            });
        } catch (error) {
            self.postMessage({ type: 'error', message: error.message || String(error) });
        } finally {
            bitmap.close();
        }
    }
};
