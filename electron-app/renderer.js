document.addEventListener('DOMContentLoaded', () => {
    const hostsTextarea = document.getElementById('hosts-content');
    const saveButton = document.getElementById('save-button');

    // 初始化textarea为空（不再加载hosts）
    hostsTextarea.value = '';
    hostsTextarea.placeholder = '输入要追加到/etc/hosts的内容';

    // 保存按钮点击事件
    saveButton.addEventListener('click', async () => {
        console.log('保存按钮点击');
        const newContent = hostsTextarea.value;
        await saveHosts();
    });

    // 新增 saveHosts 函数
    async function saveHosts() {
        const content = hostsTextarea.value;
        
        try {
            // 尝试直接保存（如果服务已安装）
            await window.hostsAPI.appendToHostsFile(content);
            alert('保存成功！');
        } catch (error) {
            console.log('服务未安装，尝试安装', error);
            
            // 安装服务（sudo-prompt会处理密码输入）
            const installResult = await window.hostsAPI.installService();
            if (!installResult.success) {
                alert(`安装失败: ${installResult.error}`);
                return;
            }
            
            // 重试保存
            try {
                await window.hostsAPI.appendToHostsFile(content);
                alert('保存成功！');
            } catch (e) {
                alert('保存失败: ' + e.message);
            }
        }
    }
});