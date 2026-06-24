<template>
  <div class="flash-manage">
    <!-- 顶部栏 -->
    <el-card shadow="never" class="header-card">
      <div class="header-bar">
        <h2>秒杀管理</h2>
        <div class="header-right">
          <el-button type="success" @click="openCreateDialog">
            <el-icon><Plus /></el-icon> 新建秒杀
          </el-button>
          <el-button @click="fetchList">
            <el-icon><Refresh /></el-icon> 刷新
          </el-button>
        </div>
      </div>
    </el-card>

    <!-- 秒杀活动表格 -->
    <el-card shadow="never">
      <el-table v-loading="loading" :data="list" border stripe style="width: 100%" empty-text="暂无秒杀活动">
        <el-table-column prop="id" label="ID" width="60" align="center" />
        <el-table-column prop="product_name" label="商品名称" min-width="150" show-overflow-tooltip />
        <el-table-column label="商品图片" width="90" align="center">
          <template #default="{ row }">
            <el-image
              v-if="row.product_image"
              :src="row.product_image"
              :preview-src-list="[row.product_image]"
              fit="cover"
              style="width: 50px; height: 50px; border-radius: 4px"
              preview-teleported
            />
            <el-icon v-else :size="30" color="#C0C4CC"><Picture /></el-icon>
          </template>
        </el-table-column>
        <el-table-column label="原价" width="90" align="center">
          <template #default="{ row }">¥{{ row.original_price || '-' }}</template>
        </el-table-column>
        <el-table-column label="秒杀价" width="100" align="center">
          <template #default="{ row }">
            <span style="color: #F56C6C; font-weight: bold">¥{{ row.flash_price }}</span>
          </template>
        </el-table-column>
        <el-table-column label="库存" width="100" align="center">
          <template #default="{ row }">
            {{ row.flash_stock }} / {{ row.queue_cap || '不限' }}
          </template>
        </el-table-column>
        <el-table-column label="开始时间" width="170" align="center" show-overflow-tooltip />
        <el-table-column label="结束时间" width="170" align="center" show-overflow-tooltip />
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">
              {{ statusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" align="center" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 0"
              type="success"
              size="small"
              link
              @click="handleWarmup(row)"
            >
              预热
            </el-button>
            <el-button
              v-if="row.status === 1"
              type="danger"
              size="small"
              link
              @click="handleEnd(row)"
            >
              结束
            </el-button>
            <el-button
              v-if="row.status === 0"
              type="info"
              size="small"
              link
              @click="openEditDialog(row)"
            >
              编辑
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新建/编辑秒杀弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="600px"
      :close-on-click-modal="false"
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="100px">
        <el-form-item label="选择商品" prop="product_id">
          <el-select
            v-model="form.product_id"
            filterable
            remote
            reserve-keyword
            placeholder="请输入商品名称搜索"
            :remote-method="searchProducts"
            :loading="productSearchLoading"
            clearable
            style="width: 100%"
          >
            <el-option
              v-for="p in productOptions"
              :key="p.id"
              :label="`${p.name}（¥${p.price} / 库存${p.stock}）`"
              :value="p.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="秒杀价格" prop="flash_price">
          <el-input-number v-model="form.flash_price" :min="0.01" :precision="2" :step="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="秒杀库存" prop="flash_stock">
          <el-input-number v-model="form.flash_stock" :min="1" :step="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="排队上限" prop="queue_cap">
          <el-input-number v-model="form.queue_cap" :min="0" :step="1" style="width: 100%" />
          <div class="form-tip">0 表示不限制排队人数</div>
        </el-form-item>
        <el-form-item label="开始时间" prop="start_time">
          <el-date-picker
            v-model="form.start_time"
            type="datetime"
            placeholder="选择开始时间"
            format="YYYY-MM-DD HH:mm:ss"
            value-format="YYYY-MM-DD HH:mm:ss"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="结束时间" prop="end_time">
          <el-date-picker
            v-model="form.end_time"
            type="datetime"
            placeholder="选择结束时间"
            format="YYYY-MM-DD HH:mm:ss"
            value-format="YYYY-MM-DD HH:mm:ss"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import api from '../../api'
import { ElMessage, ElMessageBox } from 'element-plus'

// ==================== 数据状态 ====================
const loading = ref(false)
const list = ref([])

onMounted(() => {
  fetchList()
})

// 获取秒杀列表
async function fetchList() {
  loading.value = true
  try {
    const res = await api.get('/admin/flash/list')
    list.value = res.data || []
  } catch (e) {
    list.value = []
  } finally {
    loading.value = false
  }
}

// ==================== 状态映射 ====================
function statusType(status) {
  const map = { 0: 'info', 1: 'success', 2: '', 3: 'danger' }
  return map[status] || 'info'
}

function statusText(status) {
  const map = { 0: '未开始', 1: '进行中', 2: '已结束', 3: '已取消' }
  return map[status] || '未知'
}

// ==================== 新建/编辑弹窗 ====================
const dialogVisible = ref(false)
const submitLoading = ref(false)
const formRef = ref(null)
const isEdit = ref(false)
const editId = ref(null)

const form = reactive({
  product_id: null,
  flash_price: 99,
  flash_stock: 10,
  queue_cap: 0,
  start_time: '',
  end_time: ''
})

const formRules = {
  product_id: [{ required: true, message: '请选择商品', trigger: 'change' }],
  flash_price: [{ required: true, message: '请输入秒杀价', trigger: 'blur' }],
  flash_stock: [{ required: true, message: '请输入秒杀库存', trigger: 'blur' }],
  start_time: [{ required: true, message: '请选择开始时间', trigger: 'change' }],
  end_time: [{ required: true, message: '请选择结束时间', trigger: 'change' }]
}

const dialogTitle = computed(() => (isEdit.value ? '编辑秒杀' : '新建秒杀'))

// 商品搜索
const productSearchLoading = ref(false)
const productOptions = ref([])

async function searchProducts(query) {
  if (!query || query.length < 1) {
    productOptions.value = []
    return
  }
  productSearchLoading.value = true
  try {
    const res = await api.get('/product/list', {
      params: {
        keyword: query,
        page_num: 1,
        page_size: 20
      }
    })
    productOptions.value = res.data?.list || []
  } catch (e) {
    productOptions.value = []
  } finally {
    productSearchLoading.value = false
  }
}

// 打开新建弹窗
function openCreateDialog() {
  isEdit.value = false
  editId.value = null
  resetFormData()
  dialogVisible.value = true
}

// 打开编辑弹窗
function openEditDialog(row) {
  isEdit.value = true
  editId.value = row.id
  form.product_id = row.product_id
  form.flash_price = row.flash_price
  form.flash_stock = row.flash_stock
  form.queue_cap = row.queue_cap || 0
  form.start_time = row.start_time
  form.end_time = row.end_time

  // 预填当前商品到选项
  if (row.product_name) {
    productOptions.value = [{
      id: row.product_id,
      name: row.product_name,
      price: row.original_price || 0,
      stock: 0
    }]
  }

  dialogVisible.value = true
}

// 重置表单
function resetFormData() {
  form.product_id = null
  form.flash_price = 99
  form.flash_stock = 10
  form.queue_cap = 0
  form.start_time = ''
  form.end_time = ''
  productOptions.value = []
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

function resetForm() {
  resetFormData()
  isEdit.value = false
  editId.value = null
}

// 提交
async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  // 验证结束时间大于开始时间
  if (form.start_time >= form.end_time) {
    ElMessage.warning('结束时间必须晚于开始时间')
    return
  }

  submitLoading.value = true
  try {
    const body = {
      flash_price: form.flash_price,
      flash_stock: form.flash_stock,
      queue_cap: form.queue_cap,
      start_time: form.start_time,
      end_time: form.end_time
    }
    if (!isEdit.value) {
      body.product_id = form.product_id
    }

    if (isEdit.value) {
      await api.put(`/admin/flash/${editId.value}`, body)
      ElMessage.success('秒杀活动已更新')
    } else {
      await api.post('/admin/flash', body)
      ElMessage.success('秒杀活动已创建')
    }

    dialogVisible.value = false
    fetchList()
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    submitLoading.value = false
  }
}

// ==================== 预热 ====================
async function handleWarmup(row) {
  try {
    await ElMessageBox.confirm(
      `确定要预热秒杀活动「${row.product_name || 'ID:' + row.id}」吗？预热后数据将加载到Redis缓存。`,
      '预热确认',
      {
        confirmButtonText: '确定预热',
        cancelButtonText: '取消',
        type: 'info'
      }
    )
    await api.post(`/admin/flash/${row.id}/warmup`)
    ElMessage.success('预热成功')
    fetchList()
  } catch (e) {
    if (e === 'cancel' || e === 'close') return
    // 错误已由拦截器处理
  }
}

// ==================== 结束秒杀 ====================
async function handleEnd(row) {
  try {
    await ElMessageBox.confirm(
      `确定要结束秒杀活动「${row.product_name || 'ID:' + row.id}」吗？结束后用户将无法继续抢购。`,
      '结束确认',
      {
        confirmButtonText: '确定结束',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await api.post(`/admin/flash/${row.id}/end`)
    ElMessage.success('秒杀已结束')
    fetchList()
  } catch (e) {
    if (e === 'cancel' || e === 'close') return
    // 错误已由拦截器处理
  }
}
</script>

<style scoped>
.flash-manage {
  max-width: 1400px;
  margin: 0 auto;
}

.header-card {
  margin-bottom: 16px;
}

.header-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-bar h2 {
  margin: 0;
  font-size: 20px;
}

.header-right {
  display: flex;
  gap: 10px;
}

.form-tip {
  color: #909399;
  font-size: 12px;
  margin-top: 4px;
}
</style>
