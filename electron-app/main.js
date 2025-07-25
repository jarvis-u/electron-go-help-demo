const {app, BrowserWindow, ipcMain, dialog} = require('electron')
const path = require('path')
const net = require('net')
const sudo = require('sudo-prompt')
const helper = require('ktctl-helper')
const { execSync } = require('child_process');


const SOCKET_PATH = '/var/run/com.shouqianba.ktctl.sock'
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
                width: 500,
                height: 350,
                parent: mainWindow,
                modal: true,
                webPreferences: {
                    preload: path.join(__dirname, 'preload.js'),
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
                resolve(null)
            })
        })
    })
})

app.on('window-all-closed', function () {
    if (process.platform !== 'darwin') app.quit()
})

console.log("注册IPC处理器")
ipcMain.handle('stop-debug', async () => {
    console.log("停止调试命令请求");

    return new Promise((resolve, reject) => {
        const client = net.createConnection(SOCKET_PATH, () => {
            console.log("连接到服务，发送停止调试命令");

            const request = {
                jsonrpc: "2.0",
                method: "HelperRPC.EndDebug",
                params: [],  // 空数组
                id: Date.now()
            };
            const requestStr = JSON.stringify(request) + '\n';
            console.log("发送JSON-RPC请求:", requestStr);
            client.write(requestStr);
        });

        let responseData = '';
        client.on('data', (data) => {
            responseData += data.toString();
            if (responseData.includes('\n')) {
                try {
                    const response = JSON.parse(responseData.split('\n')[0]);
                    console.log("收到停止调试响应:", response);
                    if (response.error) {
                        reject(new Error(response.error.message));
                    } else {
                        resolve(response.result);
                    }
                } catch (e) {
                    reject(e);
                } finally {
                    client.end();
                }
            }
        });

        client.on('error', (err) => {
            console.error("停止调试出错:", err);
            reject(err);
        });
    });
})

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
            console.log(`服务启动中... 重试 ${i + 1}/10`);
        }

        if (!serviceReady) {
            throw new Error('服务安装后启动失败');
        }
    }

    const hasNewVersion = await checkNewVersion();
    if (hasNewVersion) {
        console.log("服务存在新版本，触发安装流程")
        const installResult = await installService()
        if (!installResult.success) {
            throw new Error('服务安装失败')
        }

        let serviceReady = false;
        for (let i = 0; i < 10; i++) { // 增加到10次重试
            await new Promise(resolve => setTimeout(resolve, 1000)); // 增加到1秒间隔
            serviceReady = await checkServiceInstallation();
            if (serviceReady) break;
            console.log(`服务启动中... 重试 ${i + 1}/10`);
        }

        if (!serviceReady) {
            throw new Error('服务安装后启动失败');
        }
    }

    return new Promise((resolve, reject) => {
        const client = net.createConnection(SOCKET_PATH, () => {
            console.log("连接到服务执行命令");

            const request = {
                jsonrpc: "2.0",
                method: "HelperRPC.StartDebug",
                params: [command],  // 命令字符串作为数组元素
                id: Date.now()
            };
            const requestStr = JSON.stringify(request);
            console.log("发送JSON-RPC请求:", requestStr);
            client.write(requestStr);
        });

        let responseData = '';
        client.on('data', (data) => {
            responseData += data.toString();
            if (responseData.includes('\n')) {
                try {
                    const response = JSON.parse(responseData.split('\n')[0]);
                    console.log("收到JSON-RPC响应:", response);
                    if (response.error) {
                        reject(new Error(response.error.message));
                    } else {
                        resolve(response.result);
                    }
                } catch (e) {
                    reject(e);
                } finally {
                    client.end();
                }
            }
        });

        client.on('error', (err) => {
            console.error("命令执行出错:", err);
            reject(err);
        });
    });
})

function checkNewVersion() {
    return new Promise((resolve, reject) => {
        const versionInfo = execSync(helper.version()).toString().trim();
        const client = net.createConnection(SOCKET_PATH, () => {
            const request = {
                jsonrpc: "2.0",
                method: "HelperRPC.CheckNewVersion",
                params: [versionInfo],
                id: Date.now()
            };
            client.write(JSON.stringify(request) + '\n');
        });

        let responseData = '';
        client.on('data', (data) => {
            responseData += data.toString();
            if (responseData.includes('\n')) {
                try {
                    const response = JSON.parse(responseData.split('\n')[0]);
                    console.log("版本检查响应:", response);
                    if (response.error) {
                        reject(new Error(response.error.message));
                    } else {
                        resolve(response.result); // 返回bool值
                    }
                } catch (e) {
                    reject(e);
                } finally {
                    client.end();
                }
            }
        });

        client.on('error', (err) => {
            console.error("版本检查出错:", err);
            reject(err);
        });
    });
}


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
        const options = {
            name: 'Hosts Helper Service Installation',
            icns: '/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/BookmarkIcon.icns',
            env: {PATH: process.env.PATH}
        };

        sudo.exec(helper.enable(), options, (error, stdout, stderr) => {
            console.log('stdout:', stdout);
            console.log('stderr:', stderr);

            if (error) {
                console.error('安装失败:', error.message);
                resolve({success: false, error: error.message});
            } else {
                console.log('安装成功');
                resolve({success: true});
            }
        });
    });
}
