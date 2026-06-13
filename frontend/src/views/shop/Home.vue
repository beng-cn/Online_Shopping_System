<template>
  <div class="home-page">
    <!-- 搜索栏 -->
    <el-card class="search-card" shadow="never">
      <el-row :gutter="16" align="middle">
        <el-col :span="8">
          <el-input
            v-model="searchKeyword"
            placeholder="搜索商品名称"
            clearable
            prefix-icon="Search"
            @keyup.enter="handleSearch"
          />
        </el-col>
        <el-col :span="6">
          <el-cascader
            v-model="selectedCategory"
            :options="categoryOptions"
            :props="{ value: 'id', label: 'name', children: 'children', checkStrictly: true, emitPath: false }"
            placeholder="全部分类"
            clearable
            style="width: 100%"
            @change="handleSearch"
          />
        </el-col>
        <el-col :span="4">
          <el-select v-model="sortType" placeholder="排序方式" style="width: 100%" @change="handleSearch">
            <el-option label="最新上架" value="created_at" />
            <el-option label="价格升序" value="price_asc" />
            <el-option label="价格降序" value="price_desc" />
            <el-option label="销量优先" value="sales" />
          </el-select>
        </el-col>
        <el-col :span="3">
          <el-button type="primary" :icon="'Search'" @click="handleSearch">搜索</el-button>
        </el-col>
      </el-row>
    </el-card>

    <!-- 商品列表 -->
    <div v-loading="loading" class="product-grid-area">
      <el-empty v-if="!loading && productList.length === 0" description="暂无商品" />

      <div v-else class="product-grid">
        <el-card
          v-for="item in productList"
          :key="item.id"
          class="product-card"
          shadow="hover"
          @click="goDetail(item.id)"
        >
          <div class="product-image">
            <el-image
              :src="item.image_url || '/src/assets/hero.png'"
              fit="cover"
              style="width: 100%; height: 200px"
              lazy
            >
              <template #error>
                <div class="image-placeholder">
                  <el-icon :size="40"><Picture /></el-icon>
                </div>
              </template>
            </el-image>
          </div>
          <div class="product-info">
            <h3 class="product-name">{{ item.name }}</h3>
            <div class="product-meta">
              <span class="product-price">
                <span class="price-symbol">￥</span>{{ formatPrice(item.price) }}
              </span>
              <span v-if="item.sales_count !== undefined" class="product-sales">已售 {{ item.sales_count }}</span>
            </div>
            <div v-if="item.category_name" class="product-category">
              <el-tag size="small" type="info">{{ item.category_name }}</el-tag>
            </div>
          </div>
        </el-card>
      </div>
    </div>

    <!-- 分页 -->
    <div v-if="total > 0" class="pagination-wrapper">
      <el-pagination
        v-model:current-page="pageNum"
        v-model:page-size="pageSize"
        :page-sizes="[12, 20, 40]"
        :total="total"
        layout="total, sizes, prev, pager, next"
        background
        @current-change="fetchProducts"
        @size-change="fetchProducts"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '../../api'
import { ElMessage } from 'element-plus'

const router = useRouter()

// 搜索条件
const searchKeyword = ref('')
const selectedCategory = ref(null)
const sortType = ref('created_at')

// 分类级联数据
const categoryOptions = ref([])

// 商品数据
const productList = ref([])
const pageNum = ref(1)
const pageSize = ref(20)
const total = ref(0)
const loading = ref(false)

// 获取分类列表（先查父分类，再逐个查子分类）
async function fetchCategories() {
  try {
    const parentsRes = await api.get('/product/category/parents')
    const parents = parentsRes.data || []
    if (!parents.length) return

    // 逐个父分类查子分类，构建级联选项
    const options = []
    for (const p of parents) {
      try {
        const childrenRes = await api.get('/product/category/children', { params: { parent_id: p.id } })
        const children = (childrenRes.data || []).map(c => ({ id: c.id, name: c.name }))
        if (children.length) {
          options.push({ id: p.id, name: p.name, children })
        }
      } catch (e) {
        // 单个父分类的子分类加载失败不影响整体
      }
    }
    categoryOptions.value = options
  } catch (e) {
    console.error('获取分类失败:', e)
  }
}

// 获取商品列表
async function fetchProducts() {
  loading.value = true
  try {
    const params = {
      keyword: searchKeyword.value || '',
      page_num: pageNum.value,
      page_size: pageSize.value,
      sort: mapSortField(sortType.value),
    }
    // 只有选中分类时才传 category_id
    if (selectedCategory.value) {
      params.category_id = String(selectedCategory.value)
    }
    const res = await api.post('/product/list', params)
    const data = res.data
    productList.value = data.list || data.products || []
    total.value = data.total || 0
  } catch (e) {
    productList.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 排序字段映射：前端选择值 -> 后端sort参数值
function mapSortField(val) {
  const map = {
    created_at: 'created_at',
    price_asc: 'price_asc',
    price_desc: 'price_desc',
    sales: 'sales',
  }
  return map[val] || 'created_at'
}

// 搜索（重置到第一页）
function handleSearch() {
  pageNum.value = 1
  fetchProducts()
}

// 格式化价格（保留两位小数）
function formatPrice(price) {
  if (price === undefined || price === null) return '0.00'
  return Number(price).toFixed(2)
}

// 跳转到商品详情
function goDetail(id) {
  router.push(`/product/${id}`)
}

onMounted(() => {
  fetchCategories()
  fetchProducts()
})
</script>

<style scoped>
.home-page {
  max-width: 1400px;
  margin: 0 auto;
}

.search-card {
  margin-bottom: 20px;
}

.product-grid-area {
  min-height: 400px;
}

.product-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 16px;
}

.product-card {
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.product-card:hover {
  transform: translateY(-2px);
}

.product-image {
  width: 100%;
  height: 200px;
  overflow: hidden;
  border-radius: 4px;
  background: #f5f7fa;
}

.image-placeholder {
  width: 100%;
  height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
  color: #c0c4cc;
}

.product-info {
  padding: 12px 0 0;
}

.product-name {
  font-size: 15px;
  font-weight: 500;
  color: #303133;
  margin-bottom: 8px;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  height: 42px;
}

.product-meta {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 6px;
}

.product-price {
  font-size: 20px;
  font-weight: bold;
  color: #f56c6c;
}

.price-symbol {
  font-size: 14px;
}

.product-sales {
  font-size: 12px;
  color: #909399;
}

.product-category {
  margin-top: 4px;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 24px;
  padding-bottom: 20px;
}
</style>
