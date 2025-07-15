const { app, BrowserWindow, ipcMain, dialog } = require('electron')
const path = require('path')
const net = require('net')
const sudo = require('sudo-prompt')

const SOCKET_PATH = '/var/run/com.example.hostshelper.sock'
let mainWindow

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
}

app.whenReady().then(() => {
  createWindow()
  
  app.on('activate', function () {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })

  ipcMain.handle('open-command-dialog', async (event) => {
    console.log("打开命令输入对话框")
    
    return new Promise((resolve) => {
      const dialogWindow = new BrowserWindow({
        width: 400,
        height: 250,
        parent: mainWindow,
        modal: true,
        webPreferences: {
          nodeIntegration: true,
          contextIsolation: false
        }
      })
      
      dialogWindow.loadFile('command-dialog.html')
      
      // 监听对话框响应
      ipcMain.once('command-dialog-response', (event, command) => {
        resolve(command)
        dialogWindow.close()
      })
      
      // 对话框关闭时返回null
      dialogWindow.on('closed', () => {
        if (!dialogWindow.isDestroyed()) {
          resolve(null)
        }
      })
    })
  })
})

app.on('window-all-closed', function () {
  if (process.platform !== 'darwin') app.quit()
})

console.log("注册IPC处理器")

ipcMain.handle('append-to-hosts', async (event, content) => {
  console.log("处理append-to-hosts请求")
  
  const serviceInstalled = await checkServiceInstallation()
  if (!serviceInstalled) {
    console.log("服务未安装，触发安装流程")
    const installResult = await installService()
    if (!installResult.success) {
      throw new Error('服务安装失败')
    }

    let serviceReady = false;
    for (let i = 0; i < 5; i++) {
      await new Promise(resolve => setTimeout(resolve, 500));
      serviceReady = await checkServiceInstallation();
      if (serviceReady) break;
      console.log(`服务启动中... 重试 ${i+1}/5`);
    }
    
    if (!serviceReady) {
      throw new Error('服务安装后启动失败');
    }
  }

  return new Promise((resolve, reject) => {
    const client = net.createConnection(SOCKET_PATH, () => {
      console.log("连接到服务")
      client.write('u')
      
      const lenBuf = Buffer.alloc(4)
      lenBuf.writeUInt32BE(content.length)
      client.write(lenBuf)
      
      client.write(content)
    })

    client.on('data', (data) => {
      console.log("追加成功确认")
      resolve()
      client.end()
    })

    client.on('error', (err) => {
      console.error("append-to-hosts请求出错:", err)
      reject(err)
    })
  })
})

// 注册install-service IPC处理器
ipcMain.handle('install-service', async () => {
  return installService();
});

// 新增sudo命令执行处理器（通过特权助手服务）
ipcMain.handle('sudo-command', async (event, command) => {
  console.log("执行特权命令:", command);
  
  const serviceInstalled = await checkServiceInstallation()
  if (!serviceInstalled) {
    console.log("服务未安装，触发安装流程")
    const installResult = await installService()
    if (!installResult.success) {
      throw new Error('服务安装失败')
    }

    let serviceReady = false;
    for (let i = 0; i < 10; i++) { // 增加到10次重试
      await new Promise(resolve => setTimeout(resolve, 1000)); // 增加到1秒间隔
      serviceReady = await checkServiceInstallation();
      if (serviceReady) break;
      console.log(`服务启动中... 重试 ${i+1}/10`);
    }
    
    if (!serviceReady) {
      throw new Error('服务安装后启动失败');
    }
  }

  return new Promise((resolve, reject) => {
    const client = net.createConnection(SOCKET_PATH, () => {
      console.log("连接到服务执行命令")
      
      // 发送操作码'c'表示执行命令
      client.write('c')
      
      // 发送命令长度
      const lenBuf = Buffer.alloc(4)
      lenBuf.writeUInt32BE(command.length)
      client.write(lenBuf)
      
      // 发送命令内容
      client.write(command)
    })

    let resultBuffer = Buffer.alloc(0)
    let resultLength = -1
    
    client.on('data', (data) => {
      resultBuffer = Buffer.concat([resultBuffer, data])
      

      while (true) {
        if (resultLength === -1 && resultBuffer.length >= 4) {
          resultLength = resultBuffer.readUInt32BE(0)
          resultBuffer = resultBuffer.slice(4)
        }

        if (resultLength !== -1 && resultBuffer.length >= resultLength) {
          const result = resultBuffer.slice(0, resultLength).toString()
          resolve({ stdout: result, stderr: '' })
          client.end()
          return
        }

        break;
      }
    })

    client.on('error', (err) => {
      console.error("命令执行出错:", err)
      reject(err)
    })
    
    client.on('end', () => {
      if (resultLength === -1) {
        reject(new Error('命令执行未返回结果'))
      }
    })
  })
})

function checkServiceInstallation() {
  return new Promise((resolve) => {
    const testClient = net.createConnection(SOCKET_PATH, () => {
      testClient.end()
      resolve(true)
    })

    testClient.on('error', () => {
      resolve(false)
    })
    
    setTimeout(() => {
      testClient.destroy()
      resolve(false)
    }, 1000)
  })
}

function installService() {
  return new Promise((resolve) => {
    const helperPath = path.join(__dirname, 'hosts-helper');
    console.log("执行安装命令");
    console.log("Helper路径:", helperPath);
    
    const options = {
      name: 'Hosts Helper Service Installation',
      icns: '/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/BookmarkIcon.icns',
      env: { PATH: process.env.PATH }
    };
    
    const command = `"${helperPath}" install`;
    console.log("执行命令:", command);
    
    sudo.exec(command, options, (error, stdout, stderr) => {
      console.log('stdout:', stdout);
      console.log('stderr:', stderr);
      
      if (error) {
        console.error('安装失败:', error.message);
        resolve({ success: false, error: error.message });
      } else {
        console.log('安装成功');
        resolve({ success: true });
      }
    });
  });
}
