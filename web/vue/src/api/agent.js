import httpClient from '../utils/httpClient'

export default {
  approveDevice (userCode, callback) {
    httpClient.post('/agent/v1/auth/device/approve', {user_code: userCode}, callback)
  },

  devices (callback) {
    httpClient.get('/agent/v1/auth/devices', {}, callback)
  },

  revokeDevice (deviceId, callback) {
    httpClient.delete(`/agent/v1/auth/devices/${deviceId}`, {}, callback)
  }
}
