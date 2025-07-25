document.addEventListener('DOMContentLoaded', () => {
    const debugButton = document.getElementById('debug-button');
    const stopDebugButton = document.getElementById('stop-debug-button');

    if (!debugButton || !stopDebugButton) {
        console.error('未找到调试按钮');
        return;
    }

    debugButton.addEventListener('click', async () => {
        console.log('本地调试按钮点击');

        try {
            const command = await window.hostsAPI.openCommandDialog();
            if (!command) {
                console.log('用户取消了命令输入');
                return;
            }

            if (!command.trim()) {
                alert('命令不能为空');
                return;
            }

            const result = await window.hostsAPI.executeSudoCommand(command);
            alert(result);
        } catch (error) {
            console.error('命令执行出错:', error);
            alert(`命令执行失败: ${error}`);
        }
    });

    stopDebugButton.addEventListener('click', async () => {
        console.log('结束调试按钮点击');
        try {
            const result = await window.hostsAPI.stopDebug();
            alert(result);
        } catch (err) {
            console.error('结束调试出错:', err);
            alert('结束调试失败: ' + err.message);
        }
    });

    console.log('DOMContentLoaded完成');
});