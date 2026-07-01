<template>
  <div class="flash-page">
    <div class="back-bar">
      <el-button text @click="$router.back()">
        <el-icon><ArrowLeft /></el-icon> 返回上一级
      </el-button>
    </div>
    <div class="flash-header">
      <h2>秒杀活动 <el-tag type="danger" size="large">限时抢购</el-tag></h2>
      <p class="flash-subtitle">超值好物，手慢无！</p>
    </div>

    <!-- 加载状态 -->
    <div v-loading="loading" class="flash-list-area">
      <el-empty v-if="!loading && flashList.length === 0" description="暂无秒杀活动" />

      <div v-else class="flash-grid">
        <el-card
          v-for="item in flashList"
          :key="item.id"
          class="flash-card"
          shadow="hover"
        >
          <div class="flash-card-body">
            <!-- 商品图片 -->
            <div class="flash-image">
              <el-image
                :src="item.image || '/src/assets/hero.png'"
                fit="cover"
                style="width: 200px; height: 200px"
                lazy
              >
                <template #error>
                  <div class="image-placeholder">
                    <el-icon :size="40"><Picture /></el-icon>
                  </div>
                </template>
              </el-image>
            </div>

            <!-- 活动信息 -->
            <div class="flash-info">
              <h3 class="flash-product-name">{{ item.product_name }}</h3>

              <div class="flash-pricing">
                <span class="flash-price">
                  <span class="price-symbol">￥</span>{{ formatPrice(item.flash_price) }}
                </span>
                <span class="origin-price">￥{{ formatPrice(item.origin_price) }}</span>
              </div>

              <div class="flash-stats">
                <span>
                  剩余 <strong>{{ item.remaining }}</strong> /
                  总量 {{ item.flash_stock }}
                </span>
                <el-progress
                  :percentage="getProgress(item)"
                  :stroke-width="8"
                  :color="progressColor"
                  style="width: 120px"
                />
              </div>

              <!-- 状态标签与倒计时 -->
              <div class="flash-status-row">
                <el-tag :type="getStatusType(item.status)" size="default">
                  {{ getStatusText(item.status) }}
                </el-tag>

                <span v-if="item.status === 0" class="countdown-text">
                  距离开始：{{ countdownText(item) }}
                </span>
                <span v-else-if="item.status === 1" class="countdown-text ending">
                  距离结束：{{ countdownText(item) }}
                </span>
              </div>

              <!-- 排队人数 -->
              <div v-if="item.status === 1" class="queue-info">
                当前排队：{{ item.queue_count || 0 }} 人
              </div>
            </div>

            <!-- 操作按钮 -->
            <div class="flash-actions">
              <el-button
                v-if="item.status === 1"
                type="warning"
                size="large"
                :loading="entering === item.id"
                @click="handleEnter(item)"
              >
                排队入场
              </el-button>
              <el-button v-else-if="item.status === 0" type="info" size="large" disabled>
                尚未开始
              </el-button>
              <el-button v-else size="large" disabled>
                已结束
              </el-button>
            </div>
          </div>
        </el-card>
      </div>
    </div>

    <!-- 排队/抢购结果弹窗 -->
    <el-dialog
      v-model="queueDialogVisible"
      :title="admitted ? '入场成功' : '排队入场'"
      width="480px"
      :close-on-click-modal="false"
      :show-close="true"
    >
      <div class="queue-dialog-body">
        <el-result
          v-if="admitted && !captchaId && !captchaError"
          icon="success"
          title="入场成功"
          sub-title="正在加载验证码..."
        />
        <el-result
          v-else-if="admitted && captchaId"
          icon="success"
          title="入场成功"
          sub-title="请输入验证码完成抢购"
        />
        <el-result
          v-else-if="admitted && captchaError"
          icon="warning"
          title="验证码加载失败"
          sub-title="请点击下方按钮重试获取验证码"
        />
        <el-result
          v-else
          icon="warning"
          title="入场未成功"
          :sub-title="enterMessage || '当前人数已满，请稍后重试'"
        />

        <div v-if="enterQueueNumber" class="queue-number">
          您的排队序号：<strong>{{ enterQueueNumber }}</strong>
        </div>

        <!-- 验证码区域（入场成功后显示） -->
        <div v-if="admitted && captchaId" class="captcha-area">
          <div class="captcha-image-wrapper">
            <img
              :src="captchaImage"
              alt="验证码"
              class="captcha-img"
              @click="fetchCaptcha"
            />
            <el-button
              text
              type="primary"
              size="small"
              @click="fetchCaptcha"
              :loading="captchaLoading"
            >
              换一张
            </el-button>
          </div>
          <el-input
            v-model="captchaAnswer"
            placeholder="请输入验证码（不区分大小写）"
            maxlength="4"
            class="captcha-input"
            @keyup.enter="handleSnatch"
          />
        </div>

        <!-- 验证码加载失败重试区域 -->
        <div v-if="admitted && captchaError" class="captcha-area">
          <el-button type="primary" :loading="captchaLoading" @click="fetchCaptcha">
            重新获取验证码
          </el-button>
        </div>
      </div>

      <template #footer>
        <el-button @click="closeQueueDialog">关闭</el-button>
        <el-button
          v-if="admitted"
          type="danger"
          :loading="snatching"
          :disabled="!captchaId || captchaAnswer.length < 4"
          @click="handleSnatch"
        >
          立即抢购
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import api from '../../api'
import { ElMessage, ElMessageBox } from 'element-plus'

// 秒杀活动列表
const flashList = ref([])
const loading = ref(false)

// 当前操作的活动
const currentFlashId = ref(0)
const entering = ref(0) // 正在排队中的活动ID
const snatching = ref(false)

// 排队弹窗
const queueDialogVisible = ref(false)
const admitted = ref(false)
const enterMessage = ref('')
const enterQueueNumber = ref(0)

// 验证码
const captchaId = ref('')
const captchaImage = ref('')
const captchaAnswer = ref('')
const captchaLoading = ref(false)
const captchaError = ref(false)

// 倒计时定时器
let timer = null
let fetchPending = false // 防止并发重复请求

// 获取秒杀活动列表
let lastFetchTime = 0
async function fetchFlashList() {
  if (fetchPending) return
  const now = Date.now()
  if (now - lastFetchTime < 3000) return // 冷却 3 秒，防止 countdownText 重渲染触发死循环
  lastFetchTime = now
  fetchPending = true
  loading.value = true
  try {
    const res = await api.get('/flash/list')
    flashList.value = res.data || []
  } catch (e) {
    flashList.value = []
  } finally {
    loading.value = false
    fetchPending = false
  }
}

// 获取当前服务器时间
function getServerTime() {
  // 优先使用列表第一个元素的 server_time 字段
  if (flashList.value.length > 0 && flashList.value[0].server_time) {
    return new Date(flashList.value[0].server_time).getTime()
  }
  return Date.now()
}

// 计算库存消耗进度百分比
function getProgress(item) {
  if (item.flash_stock <= 0) return 100
  const used = item.flash_stock - (item.remaining || item.flash_stock)
  return Math.round((used / item.flash_stock) * 100)
}

// 进度条颜色
function progressColor(percentage) {
  if (percentage >= 80) return '#f56c6c'
  if (percentage >= 50) return '#e6a23c'
  return '#409eff'
}

// 状态文本
function getStatusText(status) {
  const map = { 0: '未开始', 1: '进行中', 2: '已结束', 3: '已取消' }
  return map[status] || '未知'
}

// 状态标签类型
function getStatusType(status) {
  const map = { 0: 'info', 1: 'danger', 2: 'info', 3: 'warning' }
  return map[status] || 'info'
}

// 格式化价格
function formatPrice(price) {
  if (price === undefined || price === null) return '0.00'
  return Number(price).toFixed(2)
}

// 倒计时文本（纯展示函数，不含副作用）
function countdownText(item) {
  const now = getServerTime()
  const targetTime = item.status === 0
    ? new Date(item.start_time).getTime()
    : new Date(item.end_time).getTime()

  const diff = targetTime - now
  if (diff <= 0) return '00:00:00'

  const hours = Math.floor(diff / 3600000)
  const minutes = Math.floor((diff % 3600000) / 60000)
  const seconds = Math.floor((diff % 60000) / 1000)

  const pad = (n) => String(n).padStart(2, '0')
  if (hours > 0) {
    return `${pad(hours)}:${pad(minutes)}:${pad(seconds)}`
  }
  return `${pad(minutes)}:${pad(seconds)}`
}

// 获取验证码
async function fetchCaptcha() {
  captchaLoading.value = true
  captchaError.value = false
  try {
    const res = await api.get('/auth/flash/captcha')
    const data = res.data
    captchaId.value = data.captcha_id
    captchaImage.value = data.captcha_image
    captchaAnswer.value = ''
  } catch (e) {
    captchaId.value = ''        // 清除旧验证码ID，防止与 captchaError 共存导致 UI 冲突
    captchaError.value = true
    ElMessage.error('验证码加载失败，请点击下方按钮重试')
  } finally {
    captchaLoading.value = false
  }
}

// 关闭弹窗并重置状态
function closeQueueDialog() {
  queueDialogVisible.value = false
  captchaId.value = ''
  captchaImage.value = ''
  captchaAnswer.value = ''
  admitted.value = false
  enterMessage.value = ''
  enterQueueNumber.value = 0
}

// 排队入场
async function handleEnter(item) {
  entering.value = item.id
  currentFlashId.value = item.id
  // 重置验证码状态
  captchaId.value = ''
  captchaImage.value = ''
  captchaAnswer.value = ''
  try {
    const res = await api.post('/auth/flash/enter', { flash_sale_id: item.id })
    const data = res.data
    admitted.value = data.admitted
    enterMessage.value = data.message || ''
    enterQueueNumber.value = data.queue_number

    if (data.admitted) {
      ElMessage.success('入场成功，请立即抢购！')
      // 自动加载验证码
      fetchCaptcha()
    } else {
      ElMessage.warning(data.message || '排队中，请耐心等待')
    }
    queueDialogVisible.value = true
  } catch (e) {
    // 错误已由拦截器处理
    entering.value = 0
  } finally {
    entering.value = 0
  }
}

// 发起抢购
async function handleSnatch() {
  if (!currentFlashId.value) return
  if (!captchaId.value || captchaAnswer.value.length < 4) {
    ElMessage.warning('请输入4位验证码')
    return
  }
  snatching.value = true
  try {
    const res = await api.post('/auth/flash/snatch', {
      flash_sale_id: currentFlashId.value,
      captcha_id: captchaId.value,
      captcha_answer: captchaAnswer.value.trim()
    })
    const data = res.data

    if (data.success) {
      ElMessage.success(`抢购成功！订单号：${data.order_no}`)
      closeQueueDialog()
      fetchFlashList()
    } else {
      const msg = data.message || '抢购失败'
      ElMessage.warning(msg)
      // 售罄/已参与等终态 → 关闭弹窗，不再刷新验证码
      if (msg.includes('售罄') || msg.includes('已参与') || msg.includes('已结束')) {
        closeQueueDialog()
      } else {
        fetchCaptcha() // 其他失败刷新验证码重试
      }
    }
  } catch (e) {
    // 网络异常等 → 不刷新验证码（可能是服务器问题，刷新也没用）
    closeQueueDialog()
  } finally {
    snatching.value = false
  }
}

// 每秒刷新倒计时，并检测倒计时归零自动刷新列表
function startCountdown() {
  timer = setInterval(() => {
    // 检测是否有活动倒计时归零（开始或结束），若有则静默刷新列表
    const now = getServerTime()
    for (const item of flashList.value) {
      const targetTime = item.status === 0
        ? new Date(item.start_time).getTime()
        : new Date(item.end_time).getTime()
      if (targetTime - now <= 0) {
        fetchFlashList()
        break // 一次只触发一次刷新
      }
    }
    // 强制触发视图更新（倒计时文本用）
    flashList.value = [...flashList.value]
  }, 1000)
}

onMounted(() => {
  fetchFlashList()
  startCountdown()
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.back-bar { margin-bottom: 8px; }
.flash-page {
  max-width: 1200px;
  margin: 0 auto;
}

.flash-header {
  text-align: center;
  margin-bottom: 24px;
}

.flash-header h2 {
  font-size: 28px;
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.flash-subtitle {
  color: #909399;
  font-size: 14px;
}

.flash-list-area {
  min-height: 400px;
}

.flash-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.flash-card {
  transition: transform 0.2s;
}

.flash-card:hover {
  transform: translateY(-1px);
}

.flash-card-body {
  display: flex;
  align-items: stretch;
  gap: 20px;
}

.flash-image {
  flex-shrink: 0;
  width: 200px;
  height: 200px;
  border-radius: 8px;
  overflow: hidden;
  background: #f5f7fa;
}

.image-placeholder {
  width: 200px;
  height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
  color: #c0c4cc;
}

.flash-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  min-width: 0;
}

.flash-product-name {
  font-size: 18px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}

.flash-pricing {
  display: flex;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 8px;
}

.flash-price {
  font-size: 28px;
  font-weight: bold;
  color: #f56c6c;
}

.flash-price .price-symbol {
  font-size: 16px;
}

.origin-price {
  font-size: 16px;
  color: #c0c4cc;
  text-decoration: line-through;
}

.flash-stats {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 14px;
  color: #606266;
  margin-bottom: 8px;
}

.flash-status-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 4px;
}

.countdown-text {
  font-size: 14px;
  color: #909399;
  font-weight: 500;
}

.countdown-text.ending {
  color: #f56c6c;
}

.queue-info {
  font-size: 13px;
  color: #909399;
}

.flash-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  padding: 0 12px;
}

.queue-dialog-body {
  text-align: center;
}

.queue-number {
  margin-top: 12px;
  font-size: 16px;
  color: #303133;
}

.captcha-area {
  margin-top: 16px;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 8px;
}

.captcha-image-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-bottom: 10px;
}

.captcha-img {
  height: 50px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  cursor: pointer;
  background: #fff;
}

.captcha-img:hover {
  border-color: #409eff;
}

.captcha-input {
  width: 100%;
}
</style>
