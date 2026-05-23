<template>
  <el-main>
    <el-table :data="devices">
      <el-table-column prop="device_name" label="设备名"></el-table-column>
      <el-table-column prop="client_type" label="客户端"></el-table-column>
      <el-table-column prop="client_version" label="版本"></el-table-column>
      <el-table-column prop="last_used_ip" label="最近 IP"></el-table-column>
      <el-table-column prop="last_used_at" label="最近使用"></el-table-column>
      <el-table-column prop="expires_at" label="过期时间"></el-table-column>
      <el-table-column label="状态">
        <template slot-scope="scope">
          <el-tag v-if="scope.row.revoked_at && scope.row.revoked_at !== '0001-01-01T00:00:00Z'" type="info">已撤销</el-tag>
          <el-tag v-else type="success">有效</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120">
        <template slot-scope="scope">
          <el-button size="mini" type="danger" @click="revoke(scope.row.device_id)">撤销</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-main>
</template>

<script>
import agentService from '../../api/agent'

export default {
  data () {
    return {
      devices: []
    }
  },
  created () {
    this.load()
  },
  methods: {
    load () {
      agentService.devices((data) => {
        this.devices = data || []
      })
    },
    revoke (deviceId) {
      this.$confirm('确认撤销该 CLI 设备授权？', '提示', {
        type: 'warning'
      }).then(() => {
        agentService.revokeDevice(deviceId, () => {
          this.$message.success('已撤销')
          this.load()
        })
      }).catch(() => {})
    }
  }
}
</script>
