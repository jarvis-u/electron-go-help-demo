document.addEventListener('DOMContentLoaded', () => {
    const hostsTextarea = document.getElementById('hosts-content');
    const saveButton = document.getElementById('save-button');
    const debugButton = document.getElementById('debug-button');
    const endButton = document.getElementById('end-button')

    hostsTextarea.value = '';
    hostsTextarea.placeholder = '输入要追加到/etc/hosts的内容';

    // 保存按钮点击事件
    saveButton.addEventListener('click', async () => {
        console.log('保存按钮点击');
        const newContent = hostsTextarea.value;
        await saveHosts();
    });

    debugButton.addEventListener('click', async () => {
        console.log('本地调试按钮点击');
        
        try {
            // 通过IPC请求打开命令输入对话框
            const command = await window.hostsAPI.openCommandDialog();
            
            if (!command) {
                console.log('用户取消了命令输入');
                return;
            }
            
            console.log('用户输入的命令:', command);
            
            if (!command.trim()) {
                alert('命令不能为空');
                return;
            }
            
            const result = await window.hostsAPI.executeSudoCommand(command);
            alert(`命令执行成功！\n输出: ${result.stdout}\n错误: ${result.stderr}`);
        } catch (error) {
            console.error('命令执行出错:', error);
            alert(`命令执行失败: ${error}`);
        }
    });

    async function saveHosts() {
        const content = hostsTextarea.value;
        
        try {
            await window.hostsAPI.appendToHostsFile(content);
            alert('保存成功！');
        } catch (error) {
            console.log('服务未安装，尝试安装', error);

            const installResult = await window.hostsAPI.installService();
            if (!installResult.success) {
                alert(`安装失败: ${installResult.error}`);
                return;
            }

            try {
                await window.hostsAPI.appendToHostsFile(content);
                alert('保存成功！');
            } catch (e) {
                alert('保存失败: ' + e.message);
            }
        }
    }
    
    // 添加调试信息
    console.log('DOMContentLoaded完成');
    console.log('获取到的debug按钮:', debugButton);
});