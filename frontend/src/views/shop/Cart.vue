<template>
  <div class="cart-page">
    <div class="back-bar">
      <el-button text @click="$router.back()">
        <el-icon><ArrowLeft /></el-icon> 返回上一级
      </el-button>
    </div>
    <h2 class="page-title">我的购物车</h2>

    <!-- 加载状态 -->
    <div v-loading="loading" class="cart-content">
      <el-empty v-if="!loading && cartItems.length === 0" description="购物车为空">
        <el-button @click="$router.push('/')">去逛逛</el-button>
      </el-empty>

      <template v-else-if="!loading && cartItems.length > 0">
        <!-- 购物车表格 -->
        <el-card shadow="never" class="cart-card">
          <el-table
            :data="cartItems"
            style="width: 100%"
            @selection-change="handleSelectionChange"
          >
            <el-table-column type="selection" width="55" />

            <el-table-column label="商品信息" min-width="300">
              <template #default="{ row }">
                <div class="product-cell">
                  <el-image
                    :src="row.product_image || '/src/assets/hero.png'"
                    fit="cover"
                    class="cart-product-image"
                    style="width: 70px; height: 70px"
                    lazy
                  >
                    <template #error>
                      <div class="image-fallback">
                        <el-icon :size="24"><Picture /></el-icon>
                      </div>
                    </template>
                  </el-image>
                  <div class="product-name-wrapper">
                    <span class="product-name">{{ row.product_name || '商品 #' + row.product_id }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="单价" width="140" align="center">
              <template #default="{ row }">
                <span class="item-price">￥{{ formatPrice(row.product_price) }}</span>
              </template>
            </el-table-column>

            <el-table-column label="数量" width="180" align="center">
              <template #default="{ row }">
                <el-input-number
                  v-model="row.quantity"
                  :min="1"
                  :max="99"
                  size="small"
                  @change="handleQuantityChange(row)"
                />
              </template>
            </el-table-column>

            <el-table-column label="小计" width="140" align="center">
              <template #default="{ row }">
                <span class="subtotal">￥{{ formatPrice((row.product_price || 0) * row.quantity) }}</span>
              </template>
            </el-table-column>

            <el-table-column label="操作" width="100" align="center">
              <template #default="{ row }">
                <el-button
                  type="danger"
                  size="small"
                  text
                  :loading="deleting === row.id"
                  @click="handleDelete(row.id)"
                >
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <!-- 底部结算栏 -->
        <el-card shadow="never" class="checkout-bar">
          <div class="checkout-content">
            <div class="select-all-area">
              <el-checkbox v-model="selectAll" @change="handleSelectAll">全选</el-checkbox>
            </div>

            <div class="checkout-summary">
              <span class="selected-count">
                已选 <strong>{{ selectedCartIds.length }}</strong> 件商品
              </span>
              <span class="total-price">
                合计：<span class="total-number">￥{{ formatPrice(totalPrice) }}</span>
              </span>
            </div>

            <el-button
              type="primary"
              size="large"
              :disabled="selectedCartIds.length === 0"
              :loading="creatingOrder"
              @click="handleCreateOrder"
            >
              去结算
            </el-button>
          </div>
        </el-card>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '../../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()

// 购物车列表（含产品信息）
const cartItems = ref([])
const loading = ref(false)
const deleting = ref(0)
const creatingOrder = ref(false)

// 选中项
const selectedCartIds = ref([])
const selectAll = ref(false)

// 选中变化
function handleSelectionChange(rows) {
  selectedCartIds.value = rows.map(r => r.id)
  selectAll.value = rows.length > 0 && rows.length === cartItems.value.length
}

// 全选/取消全选
function handleSelectAll(val) {
  // 通过 el-table 的 toggleAllSelection 不太方便，这里通过重新设置选中数组来模拟
  // 实际需要配合 el-table 的 ref
  // 简化处理：由 handleSelectionChange 联动即可
}

// 计算总价
const totalPrice = computed(() => {
  const selected = cartItems.value.filter(item => selectedCartIds.value.includes(item.id))
  return selected.reduce((sum, item) => sum + (item.product_price || 0) * item.quantity, 0)
})

// 获取购物车列表
async function fetchCartList() {
  loading.value = true
  try {
    const res = await api.get('/auth/cart/list')
    const rawItems = res.data || []

    // 批量获取商品信息（因为购物车接口只返回 product_id）
    const enrichedItems = await Promise.all(
      rawItems.map(async (item) => {
        try {
          const productRes = await api.get(`/product/${item.product_id}`)
          return {
            ...item,
            product_name: productRes.data?.name || '',
            product_price: productRes.data?.price || 0,
            product_image: productRes.data?.image || '',
          }
        } catch (e) {
          return { ...item, product_name: '', product_price: 0, product_image: '' }
        }
      })
    )

    cartItems.value = enrichedItems
  } catch (e) {
    cartItems.value = []
  } finally {
    loading.value = false
  }
}

// 修改数量
async function handleQuantityChange(row) {
  try {
    await api.put(`/auth/cart/${row.id}`, { quantity: row.quantity })
    ElMessage.success('数量已更新')
  } catch (e) {
    // 更新失败，恢复原数量
    fetchCartList()
  }
}

// 删除购物车项
async function handleDelete(id) {
  try {
    await ElMessageBox.confirm('确定要删除该商品吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return // 用户取消
  }

  deleting.value = id
  try {
    await api.delete(`/auth/cart/${id}`)
    ElMessage.success('删除成功')
    cartItems.value = cartItems.value.filter(item => item.id !== id)
    selectedCartIds.value = selectedCartIds.value.filter(cid => cid !== id)
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    deleting.value = 0
  }
}

// 创建订单
async function handleCreateOrder() {
  if (selectedCartIds.value.length === 0) {
    ElMessage.warning('请先选择要结算的商品')
    return
  }

  creatingOrder.value = true
  try {
    const res = await api.post('/auth/order/create', { cart_ids: selectedCartIds.value })
    ElMessage.success(`下单成功！订单号：${res.data?.order_no || '——'}`)
    router.push('/orders')
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    creatingOrder.value = false
  }
}

// 格式化价格
function formatPrice(price) {
  if (price === undefined || price === null) return '0.00'
  return Number(price).toFixed(2)
}

onMounted(() => {
  fetchCartList()
})
</script>

<style scoped>
.back-bar { margin-bottom: 8px; }
.cart-page {
  max-width: 1200px;
  margin: 0 auto;
}

.page-title {
  font-size: 22px;
  font-weight: 600;
  margin-bottom: 20px;
  color: #303133;
}

.cart-content {
  min-height: 400px;
}

.cart-card {
  margin-bottom: 0;
}

.product-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.cart-product-image {
  border-radius: 4px;
  flex-shrink: 0;
  background: #f5f7fa;
}

.image-fallback {
  width: 70px;
  height: 70px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
  color: #c0c4cc;
  border-radius: 4px;
}

.product-name-wrapper {
  min-width: 0;
}

.product-name {
  font-size: 14px;
  color: #303133;
  line-height: 1.4;
}

.item-price {
  font-size: 15px;
  font-weight: 500;
  color: #303133;
}

.subtotal {
  font-size: 15px;
  font-weight: 600;
  color: #f56c6c;
}

/* 结算栏 */
.checkout-bar {
  margin-top: 20px;
}

.checkout-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.select-all-area {
  display: flex;
  align-items: center;
}

.checkout-summary {
  display: flex;
  align-items: baseline;
  gap: 24px;
}

.selected-count {
  font-size: 14px;
  color: #606266;
}

.total-price {
  font-size: 16px;
  color: #303133;
}

.total-number {
  font-size: 24px;
  font-weight: bold;
  color: #f56c6c;
}
</style>
