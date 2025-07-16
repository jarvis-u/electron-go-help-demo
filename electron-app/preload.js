const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('hostsAPI', {
  executeSudoCommand: (command) => ipcRenderer.invoke('sudo-command', command),
  openCommandDialog: () => ipcRenderer.invoke('open-command-dialog'),
  stopDebug: () => ipcRenderer.invoke('stop-debug')
})