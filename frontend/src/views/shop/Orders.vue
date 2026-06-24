<template>
  <div class="orders-page">
    <div class="back-bar">
      <el-button text @click="$router.back()">
        <el-icon><ArrowLeft /></el-icon> 返回上一级
      </el-button>
    </div>
    <h2 class="page-title">我的订单</h2>

    <div v-loading="loading" class="orders-content">
      <el-empty v-if="!loading && orderList.length === 0" description="暂无订单">
        <el-button type="primary" @click="$router.push('/')">去逛逛</el-button>
      </el-empty>

      <template v-else-if="!loading && orderList.length > 0">
        <el-card shadow="never">
          <el-table :data="orderList" style="width:100%" row-key="id" :cell-style="{padding:'12px 8px'}">
            <el-table-column label="订单号" prop="order_no" min-width="220" />

            <el-table-column label="金额" min-width="100" align="center">
              <template #default="{ row }"><span class="order-amount">￥{{ formatPrice(row.total) }}</span></template>
            </el-table-column>

            <el-table-column label="下单时间" min-width="170" align="center">
              <template #default="{ row }"><span class="order-time">{{ formatTime(row.created_at) }}</span></template>
            </el-table-column>

            <el-table-column label="支付" min-width="110" align="center">
              <template #default="{ row }">
                <el-button v-if="row.status === 0" type="warning" size="small" :loading="row._paying" @click="handlePay(row)">🛒 去支付</el-button>
                <el-tag v-else-if="row.status === 1" type="success">已支付</el-tag>
                <span v-else style="color:#c0c4cc;font-size:13px">—</span>
              </template>
            </el-table-column>

            <el-table-column label="详情" min-width="80" align="center">
              <template #default="{ row }">
                <el-button type="info" size="small" @click="showDetail(row)">查看</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </template>
    </div>

    <!-- 订单详情弹窗 -->
    <el-dialog v-model="dialogVisible" :title="'订单详情 — ' + detailOrder?.order_no" width="600px">
      <el-table v-loading="detailLoading" :data="detailItems" border>
        <el-table-column label="商品名称" prop="name" min-width="200" />
        <el-table-column label="单价" min-width="100" align="center">
          <template #default="{ row: item }">￥{{ formatPrice(item.price) }}</template>
        </el-table-column>
        <el-table-column label="数量" min-width="60" align="center" prop="quantity" />
        <el-table-column label="小计" min-width="100" align="center">
          <template #default="{ row: item }"><span style="color:#f56c6c;font-weight:500">￥{{ formatPrice(item.price * item.quantity) }}</span></template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="dialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../../api'

const orderList = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const detailOrder = ref(null)
const detailItems = ref([])
const detailLoading = ref(false)

async function fetchOrderList() {
  loading.value = true
  try {
    const res = await api.get('/auth/order/list')
    orderList.value = (res.data || []).map(order => ({
      ...order, _paying: false,
    }))
  } catch (e) {
    orderList.value = []
  } finally {
    loading.value = false
  }
}

async function showDetail(row) {
  detailOrder.value = row
  detailItems.value = []
  dialogVisible.value = true
  detailLoading.value = true
  try {
    const res = await api.get(`/auth/order/items/${row.id}`)
    detailItems.value = res.data || []
  } catch (e) {
    detailItems.value = []
  } finally {
    detailLoading.value = false
  }
}

async function handlePay(row) {
  row._paying = true
  try {
    const res = await api.post('/auth/order/alipay', { order_id: row.id })
    const payUrl = res.data?.pay_url || res.data
    if (!payUrl) { ElMessage.error('获取支付链接失败'); return }
    window.open(payUrl, '_blank')
    ElMessage.success('已打开支付页面，请在支付宝中完成支付')
  } catch (e) { /* 拦截器已处理 */ } finally { row._paying = false }
}

function formatPrice(p) { return p != null ? Number(p).toFixed(2) : '0.00' }

function formatTime(t) {
  if (!t) return '--'
  try {
    const d = new Date(t)
    if (isNaN(d.getTime())) return t
    const pad = n => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  } catch { return t }
}

onMounted(() => fetchOrderList())
</script>

<style scoped>
.back-bar { margin-bottom: 8px; }
.orders-page { max-width: 1200px; margin: 0 auto; }
.page-title { font-size: 22px; font-weight: 600; margin-bottom: 20px; color: #303133; }
.orders-content { min-height: 400px; }
.order-amount { font-size: 15px; font-weight: 600; color: #f56c6c; }
.order-time { font-size: 13px; color: #909399; }
</style>
