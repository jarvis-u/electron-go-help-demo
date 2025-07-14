const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('hostsAPI', {
  installService: () => ipcRenderer.invoke('install-service'),
  appendToHostsFile: (content) => ipcRenderer.invoke('append-to-hosts', content)
})