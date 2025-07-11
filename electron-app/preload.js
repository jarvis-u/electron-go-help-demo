const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('hostsAPI', {
  requestSudoPassword: () => ipcRenderer.invoke('request-sudo-password'),
  installService: (password) => ipcRenderer.invoke('install-service', password),
  loadHostsFile: () => ipcRenderer.invoke('load-hosts'),
  saveHostsFile: (content) => ipcRenderer.invoke('save-hosts', content)
})