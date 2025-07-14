const { app, BrowserWindow, ipcMain } = require('electron')
const { exec } = require('child_process')
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
    
    const recheck = await checkServiceInstallation()
    if (!recheck) {
      throw new Error('服务安装后仍然不可用')
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
  const helperPath = path.join(__dirname, 'hosts-helper')
  console.log("执行安装命令")
  console.log("Helper路径:", helperPath)
  
  return new Promise((resolve) => {
    const options = {
      name: 'Hosts Helper Service Installation',
      icns: '/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/BookmarkIcon.icns',
      env: { PATH: process.env.PATH }
    }
    
    const command = `"${helperPath}" install`
    console.log("执行命令:", command)
    
    sudo.exec(command, options, (error, stdout, stderr) => {
      console.log('stdout:', stdout)
      console.log('stderr:', stderr)
      
      if (error) {
        console.error('安装失败:', error.message)
        resolve({ success: false, error: error.message })
      } else {
        console.log('安装成功')
        resolve({ success: true })
      }
    })
  })
}