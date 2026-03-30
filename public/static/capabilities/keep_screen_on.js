(
    function () {
        let wakeLock = null;

        // 申请常亮
        async function requestWakeLock() {
            try {
                wakeLock = await navigator.wakeLock.request('screen');
                console.log('Screen Wake Lock is active');

                // 监听释放事件
                wakeLock.addEventListener('release', () => {
                    console.log('Screen Wake Lock is released');
                });
            } catch (err) {
                console.error(`${err.name}, ${err.message}`);
            }
        }

        // 释放常亮
        function releaseWakeLock() {
            if (wakeLock !== null) {
                wakeLock.release();
                wakeLock = null;
            }
        }

        // 在需要常亮时调用
        requestWakeLock();

        // 当页面不再需要常亮或页面切换到后台时释放
        document.addEventListener('visibilitychange', () => {
            if (wakeLock !== null && document.visibilityState === 'visible') {
                requestWakeLock();
            }
        });

    }
)()