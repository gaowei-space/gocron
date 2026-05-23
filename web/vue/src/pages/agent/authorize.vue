<template>
  <el-container>
    <el-main>
      <el-card class="authorize-card">
        <div slot="header">
          <span>CLI 设备授权</span>
        </div>
        <div v-if="!isSuperAdmin" class="message error">
          仅超级管理员可以授权 gocron-cli。
        </div>
        <div v-else-if="success" class="message success">
          授权成功，可以回到命令行继续操作。
        </div>
        <div v-else>
          <p class="message">确认授权当前 CLI 设备访问 gocron。</p>
          <el-button type="primary" :loading="loading" @click="approve">确认授权</el-button>
        </div>
      </el-card>
    </el-main>
  </el-container>
</template>

<script>
import agentService from '../../api/agent'
import userStorage from '../../storage/user'

export default {
  data () {
    return {
      loading: false,
      success: false
    }
  },
  computed: {
    isSuperAdmin () {
      return userStorage.getIsSuperAdmin()
    },
    userCode () {
      return this.$route.query.user_code || ''
    }
  },
  methods: {
    approve () {
      if (!this.userCode) {
        this.$message.error('授权码不能为空')
        return
      }
      this.loading = true
      agentService.approveDevice(this.userCode, () => {
        this.success = true
        this.loading = false
      })
    }
  }
}
</script>

<style scoped>
.authorize-card {
  width: 420px;
  margin: 80px auto;
}
.message {
  color: #385879;
  line-height: 1.6;
}
.error {
  color: #f56c6c;
}
.success {
  color: #67c23a;
}
</style>
