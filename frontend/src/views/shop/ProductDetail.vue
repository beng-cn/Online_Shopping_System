<template>
  <div class="product-detail-page">
    <!-- 加载状态 -->
    <div v-if="loading" class="loading-wrapper">
      <el-skeleton :rows="8" animated />
    </div>

    <!-- 商品不存在 -->
    <el-empty v-else-if="!product" description="商品不存在或已下架" />

    <!-- 商品详情 -->
    <div v-else class="product-detail-container">
      <!-- 左侧：商品图片 -->
      <div class="product-gallery">
        <el-image
          :src="product.image_url || '/src/assets/hero.png'"
          fit="cover"
          class="main-image"
        >
          <template #error>
            <div class="image-placeholder">
              <el-icon :size="60"><Picture /></el-icon>
            </div>
          </template>
        </el-image>
      </div>

      <!-- 右侧：商品信息 -->
      <div class="product-info-panel">
        <el-card shadow="never" class="info-card">
          <!-- 分类标签 -->
          <div v-if="product.category_name" class="category-tag">
            <el-tag type="info" size="default">{{ product.category_name }}</el-tag>
          </div>

          <h1 class="product-name">{{ product.name }}</h1>

          <!-- 价格区 -->
          <div class="price-section">
            <span class="current-price">
              <span class="price-symbol">￥</span>{{ formatPrice(product.price) }}
            </span>
            <span v-if="product.origin_price && product.origin_price > product.price" class="origin-price">
              ￥{{ formatPrice(product.origin_price) }}
            </span>
          </div>

          <!-- 销售信息 -->
          <div class="meta-row">
            <span v-if="product.sales_count !== undefined">
              销量：<strong>{{ product.sales_count }}</strong>
            </span>
            <span v-if="product.stock !== undefined">
              库存：<strong :class="product.stock <= 0 ? 'out-of-stock' : ''">
                {{ product.stock <= 0 ? '已售罄' : product.stock + '件' }}
              </strong>
            </span>
          </div>

          <!-- 数量选择 -->
          <div class="quantity-row">
            <span class="label">数量：</span>
            <el-input-number
              v-model="quantity"
              :min="1"
              :max="product.stock || 1"
              :disabled="product.stock <= 0"
              size="large"
            />
          </div>

          <!-- 操作按钮 -->
          <div class="action-row">
            <el-button
              type="warning"
              size="large"
              :disabled="product.stock <= 0"
              :loading="addingToCart"
              @click="handleAddToCart"
            >
              <el-icon><ShoppingCart /></el-icon> 加入购物车
            </el-button>
            <el-button
              type="success"
              size="large"
              :disabled="product.stock <= 0"
              :loading="buyingNow"
              @click="handleBuyNow"
            >
              立即购买
            </el-button>
          </div>

          <!-- 商品详情描述 -->
          <div v-if="product.description" class="product-description">
            <el-divider />
            <h3>商品详情</h3>
            <p>{{ product.description }}</p>
          </div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '../../stores/user'
import api from '../../api'
import { ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const product = ref(null)
const loading = ref(false)
const quantity = ref(1)
const addingToCart = ref(false)
const buyingNow = ref(false)

// 监听路由参数变化，重新加载
watch(() => route.params.id, (newId) => {
  if (newId) fetchProduct(newId)
})

// 获取商品详情
async function fetchProduct(id) {
  loading.value = true
  quantity.value = 1
  try {
    const res = await api.get(`/product/${id}`)
    product.value = res.data
  } catch (e) {
    product.value = null
  } finally {
    loading.value = false
  }
}

// 格式化价格
function formatPrice(price) {
  if (price === undefined || price === null) return '0.00'
  return Number(price).toFixed(2)
}

// 加入购物车
async function handleAddToCart() {
  if (!userStore.isLoggedIn) {
    ElMessage.warning('请先登录后再添加购物车')
    router.push('/login')
    return
  }
  if (!product.value || product.value.stock <= 0) return

  addingToCart.value = true
  try {
    await api.post('/auth/cart/add', {
      product_id: product.value.id,
      quantity: quantity.value,
    })
    ElMessage.success(`已将 ${quantity.value} 件《${product.value.name}》加入购物车`)
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    addingToCart.value = false
  }
}

// 立即购买
async function handleBuyNow() {
  if (!userStore.isLoggedIn) {
    ElMessage.warning('请先登录后再购买')
    router.push('/login')
    return
  }
  if (!product.value || product.value.stock <= 0) return

  buyingNow.value = true
  try {
    // 先加入购物车，再直接创建订单
    await api.post('/auth/cart/add', {
      product_id: product.value.id,
      quantity: quantity.value,
    })
    // 获取购物车列表，找到刚添加的
    const cartRes = await api.get('/auth/cart/list')
    const carts = cartRes.data || []
    const target = carts.find(c => c.product_id === product.value.id)
    if (target) {
      const orderRes = await api.post('/auth/order/create', { cart_ids: [target.id] })
      ElMessage.success(`下单成功！订单号：${orderRes.data?.order_no || '——'}`)
      router.push('/orders')
    } else {
      ElMessage.warning('请到购物车页面结算')
      router.push('/cart')
    }
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    buyingNow.value = false
  }
}

onMounted(() => {
  if (route.params.id) fetchProduct(route.params.id)
})
</script>

<style scoped>
.product-detail-page {
  max-width: 1200px;
  margin: 0 auto;
}

.loading-wrapper {
  padding: 40px;
}

.product-detail-container {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}

/* 左侧图片 */
.product-gallery {
  flex-shrink: 0;
  width: 480px;
}

.main-image {
  width: 480px;
  height: 480px;
  border-radius: 8px;
  background: #f5f7fa;
}

.image-placeholder {
  width: 480px;
  height: 480px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
  color: #c0c4cc;
}

/* 右侧信息 */
.product-info-panel {
  flex: 1;
  min-width: 0;
}

.info-card {
  padding: 8px;
}

.category-tag {
  margin-bottom: 8px;
}

.product-name {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 16px;
  line-height: 1.4;
}

.price-section {
  display: flex;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 16px;
  padding: 16px;
  background: #fef0f0;
  border-radius: 8px;
}

.current-price {
  font-size: 32px;
  font-weight: bold;
  color: #f56c6c;
}

.current-price .price-symbol {
  font-size: 18px;
}

.origin-price {
  font-size: 16px;
  color: #c0c4cc;
  text-decoration: line-through;
}

.meta-row {
  display: flex;
  gap: 24px;
  margin-bottom: 20px;
  font-size: 14px;
  color: #606266;
}

.out-of-stock {
  color: #f56c6c;
}

.quantity-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}

.quantity-row .label {
  font-size: 14px;
  color: #606266;
}

.action-row {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.product-description {
  color: #606266;
  line-height: 1.8;
}

.product-description h3 {
  font-size: 16px;
  color: #303133;
  margin-bottom: 12px;
}

@media (max-width: 900px) {
  .product-detail-container {
    flex-direction: column;
  }
  .product-gallery {
    width: 100%;
  }
  .main-image {
    width: 100%;
    height: auto;
    aspect-ratio: 1;
  }
  .image-placeholder {
    width: 100%;
    height: 300px;
  }
}
</style>
