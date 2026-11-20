// The N console keeps the existing device manager UI but uses the Nexus
// connection flow when a stream is started.

window.startStream = function startNexusStream(serial) {
    const device = knownDevices.find(item => item.device_id === serial);
    if (!device || !device.uuid) {
        showToast('设备缺少 UUID，无法建立连接', 'error');
        return;
    }

    const config = ensureDeviceConfig(device);
    const finalConfig = {
        ...config,
        remote_uuid: device.uuid,
        driver_config: { ...(config.driver_config || {}) }
    };
    if (config.device_type === 'android') {
        finalConfig.driver_config.deviceID = config.device_ip || config.device_id || '';
    }

    sessionStorage.setItem('webscreen_device_configs_now', JSON.stringify(finalConfig));
    showToast(i18n.t('starting_stream'));
    setTimeout(() => {
        window.location.href = `/screen/${encodeURIComponent(device.uuid)}?nexus=1`;
    }, 500);
};


