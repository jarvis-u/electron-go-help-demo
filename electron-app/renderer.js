document.addEventListener('DOMContentLoaded', () => {
    const hostsTextarea = document.getElementById('hosts-content');
    const saveButton = document.getElementById('save-button');

    // 初始化加载hosts文件内容
    window.hostsAPI.loadHostsFile()
        .then(content => {
            hostsTextarea.value = content;
        })
        .catch(err => {
            console.error('加载hosts文件失败:', err);
            hostsTextarea.value = `# 加载失败: ${err.message}`;
        });

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
            await window.hostsAPI.saveHostsFile(content);
            alert('保存成功！');
        } catch (error) {
            console.log('服务未安装，尝试安装', error);
            
            // 请求sudo密码
            const password = await window.hostsAPI.requestSudoPassword();
            if (!password) return;
            
            // 安装服务
            const installResult = await window.hostsAPI.installService(password);
            if (!installResult.success) {
                alert(`安装失败: ${installResult.error}`);
                return;
            }
            
            // 重试保存
            try {
                await window.hostsAPI.saveHostsFile(content);
                alert('保存成功！');
            } catch (e) {
                alert('保存失败: ' + e.message);
            }
        }
    }
});