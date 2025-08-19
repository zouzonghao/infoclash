import { defineStore } from 'pinia'

export const useConnectionsStore = defineStore('connections', {
  state: () => ({
    connections: [],
  }),
  actions: {
    setConnections(connections: never[]) {
      this.connections = connections
    },
  },
})