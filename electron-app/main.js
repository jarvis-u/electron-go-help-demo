const { app, BrowserWindow, ipcMain, dialog } = require('electron')
const { exec } = require('child_process')
const path = require('path')
const net = require('net')

// 定义socket路径
const SOCKET_PATH = '/var/run/com.example.hostshelper.sock'

let mainWindow

// 创建窗口函数
function createWindow() {
  mainWindow = new BrowserWindow({
    width: 800,
    height: 600,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true
    }
  })

  mainWindow.loadFile('index.html')
  // mainWindow.webContents.openDevTools()
}

app.whenReady().then(async () => {
  // 获取hosts-helper的绝对路径
  const helperPath = path.join(__dirname, 'hosts-helper');
  console.log("hosts-helper路径:", helperPath);
  
  // 检查服务是否已安装
  const serviceInstalled = await checkServiceInstallation();
  if (!serviceInstalled) {
    console.log("服务未安装，开始安装流程");
    
    // 在终端提示输入密码
    const { exec } = require('child_process');
    const { createInterface } = require('readline');
    
    const rl = createInterface({
      input: process.stdin,
      output: process.stdout
    });
    
    rl.question('请输入管理员密码以安装服务: ', (password) => {
      rl.close();
      
      if (password) {
        // 安全传递密码
        const escapedPassword = password.replace(/'/g, "'\\''");
        
        // 执行Go助手安装命令 - 使用绝对路径
        const command = `echo '${escapedPassword}' | sudo -S "${helperPath}" install`;
        console.log("执行安装命令:", command);
        
        exec(command, (error, stdout, stderr) => {
          if (error) {
            console.error('安装命令执行失败:', error);
            console.error('错误输出:', stderr);
            console.error('服务安装失败，退出应用');
            app.quit();
          } else {
            console.log('安装命令输出:', stdout);
            console.log('服务安装成功');
            createWindow();
          }
        });
      } else {
        console.log("未提供密码，退出应用");
        app.quit();
      }
    });
  } else {
    createWindow();
  }

  app.on('activate', function () {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

app.on('window-all-closed', function () {
  if (process.platform !== 'darwin') app.quit()
})

// 注册IPC处理器
console.log("注册IPC处理器");
ipcMain.handle('load-hosts', async (event) => {
  console.log("处理load-hosts请求");
  return new Promise((resolve, reject) => {
    const client = net.createConnection(SOCKET_PATH, () => {
      console.log("连接到服务");
      client.write('g'); // 发送读取命令
    });

    client.on('data', (data) => {
      console.log("收到hosts内容");
      resolve(data.toString());
      client.end();
    });

    client.on('error', (err) => {
      console.error("load-hosts请求出错:", err);
      reject(err);
    });
  });
});

ipcMain.handle('save-hosts', async (event, content) => {
  console.log("处理save-hosts请求");
  
  // 检查服务是否已安装
  const serviceInstalled = await checkServiceInstallation();
  if (!serviceInstalled) {
    console.log("服务未安装，触发安装流程");
    const installResult = await installService();
    if (!installResult.success) {
      throw new Error('服务安装失败');
    }
    
    // 安装后再次检查连接
    const recheck = await checkServiceInstallation();
    if (!recheck) {
      throw new Error('服务安装后仍然不可用');
    }
  }

  return new Promise((resolve, reject) => {
    const client = net.createConnection(SOCKET_PATH, () => {
      console.log("连接到服务");
      // 发送更新命令
      client.write('u');
      
      // 发送内容长度
      const lenBuf = Buffer.alloc(4);
      lenBuf.writeUInt32BE(content.length);
      client.write(lenBuf);
      
      // 发送内容
      client.write(content);
    });

    client.on('data', (data) => {
      console.log("保存成功确认");
      resolve();
      client.end();
    });

    client.on('error', (err) => {
      console.error("save-hosts请求出错:", err);
      reject(err);
    });
  });
});

// 检查服务是否已安装
function checkServiceInstallation() {
  return new Promise((resolve) => {
    const testClient = net.createConnection(SOCKET_PATH, () => {
      testClient.end();
      resolve(true);
    });

    testClient.on('error', () => {
      resolve(false);
    });
    
    // 设置超时防止卡住
    setTimeout(() => {
      testClient.destroy();
      resolve(false);
    }, 1000);
  });
}

// 安装服务
async function installService() {
  return new Promise((resolve) => {
    // 确保窗口在前台
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.focus();
      mainWindow.show();
    }
    
    // 通知渲染进程打开密码对话框
    mainWindow.webContents.send('open-password-dialog');
    
    // 监听密码对话框结果
    ipcMain.once('password-dialog-result', (event, password) => {
      if (password) {
        // 安全传递密码：转义特殊字符
        const escapedPassword = password.replace(/'/g, "'\\''");
        
        // 执行Go助手安装命令 - 使用绝对路径
        const command = `echo '${escapedPassword}' | sudo -S "${helperPath}" install`;
        console.log("执行安装命令:", command);
        
        const child = exec(command, (error, stdout, stderr) => {
          if (error) {
            console.error('安装命令执行失败:', error);
            console.error('stderr:', stderr);
            resolve({ success: false });
          } else {
            console.log('安装命令输出:', stdout);
            console.log('服务安装成功');
            resolve({ success: true });
          }
        });
      } else {
        resolve({ success: false });
      }
    });
  });
}

// 处理密码对话框结果
ipcMain.on('password-dialog-result', (event, password) => {
  console.log("收到密码对话框结果");
});