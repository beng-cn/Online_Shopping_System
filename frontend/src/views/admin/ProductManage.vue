<template>
  <div class="product-manage">
    <!-- 顶部操作栏 -->
    <el-card shadow="never" class="header-card">
      <div class="header-bar">
        <div class="header-left">
          <h2>商品管理</h2>
        </div>
        <div class="header-right">
          <el-button type="success" @click="openAddDialog">
            <el-icon><Plus /></el-icon> 新增商品
          </el-button>
          <el-button type="success" :loading="batchKeywordsLoading" @click="handleBatchKeywords">
            <el-icon><MagicStick /></el-icon> 批量生成关键词
          </el-button>
          <el-button @click="fetchProducts">
            <el-icon><Refresh /></el-icon> 刷新
          </el-button>
        </div>
      </div>
    </el-card>

    <!-- 搜索筛选栏 -->
    <el-card shadow="never" class="filter-card">
      <el-form :inline="true" :model="searchForm" class="filter-form">
        <el-form-item label="关键词">
          <el-input v-model="searchForm.keyword" placeholder="商品名称/关键词" clearable @clear="handleSearch" @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="分类">
          <el-select v-model="searchForm.category_id" placeholder="全部分类" clearable @change="handleSearch" style="width: 180px">
            <el-option
              v-for="cat in parentCategories"
              :key="cat.id"
              :label="cat.name"
              :value="cat.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="全部状态" clearable @change="handleSearch" style="width: 120px">
            <el-option label="上架" :value="1" />
            <el-option label="下架" :value="0" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="resetSearch">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 商品表格 -->
    <el-card shadow="never">
      <el-table
        v-loading="loading"
        :data="productList"
        border
        stripe
        style="width: 100%"
        empty-text="暂无商品数据"
      >
        <el-table-column prop="id" label="ID" width="60" align="center" />
        <el-table-column label="图片" width="100" align="center">
          <template #default="{ row }">
            <el-image
              v-if="row.image"
              :src="row.image"
              :preview-src-list="[row.image]"
              fit="cover"
              style="width: 60px; height: 60px; border-radius: 6px"
              preview-teleported
            />
            <el-icon v-else :size="40" color="#C0C4CC"><Picture /></el-icon>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="商品名称" min-width="160" show-overflow-tooltip />
        <el-table-column prop="category_name" label="所属分类" width="120" align="center" />
        <el-table-column prop="price" label="价格" width="100" align="center">
          <template #default="{ row }">
            <span style="color: #E6A23C; font-weight: bold">¥{{ row.price }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="stock" label="库存" width="80" align="center" />
        <el-table-column prop="sales" label="销量" width="80" align="center" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '上架' : '下架' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" align="center" fixed="right">
          <template #default="{ row }">
            <el-button type="info" size="small" link @click="openEditDialog(row)">编辑</el-button>
            <el-button type="danger" size="small" link @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="pagination.page_num"
          v-model:page-size="pagination.page_size"
          :page-sizes="[10, 20, 50, 100]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          background
          @size-change="handleSearch"
          @current-change="handleSearch"
        />
      </div>
    </el-card>

    <!-- 新增/编辑商品弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="640px"
      :close-on-click-modal="false"
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="所属分类" prop="category_id">
          <el-cascader
            v-model="form.category_id"
            :options="categoryOptions"
            :props="{ value: 'id', label: 'name', emitPath: false, checkStrictly: false }"
            placeholder="请选择分类"
            clearable
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="商品名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入商品名称" maxlength="100" />
        </el-form-item>
        <el-form-item label="价格" prop="price">
          <el-input-number v-model="form.price" :min="0" :precision="2" :step="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="库存" prop="stock">
          <el-input-number v-model="form.stock" :min="0" :step="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="关键词" prop="keywords">
          <el-input v-model="form.keywords" placeholder="搜索关键词，多个用逗号分隔（留空则自动生成）" maxlength="500" />
        </el-form-item>
        <el-form-item label="商品图片" prop="image">
          <div class="upload-wrap">
            <el-upload
              class="image-uploader"
              action=""
              :auto-upload="false"
              :show-file-list="false"
              :on-change="handleImageChange"
              accept="image/*"
            >
              <el-image
                v-if="form.image"
                :src="form.image"
                fit="cover"
                style="width: 148px; height: 148px; border-radius: 6px"
              />
              <el-icon v-else class="upload-icon"><Plus /></el-icon>
            </el-upload>
            <div v-if="form.image" class="image-actions">
              <el-button type="danger" size="small" @click="form.image = ''">移除图片</el-button>
            </div>
            <div class="upload-tip">
              <el-button type="info" :loading="uploading" size="small" @click="triggerUpload">上传图片</el-button>
              <span v-if="uploading" style="margin-left: 8px; color: #409EFF">上传中...</span>
            </div>
          </div>
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">上架</el-radio>
            <el-radio :value="0">下架</el-radio>
          </el-radio-group>
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
const productList = ref([])

// 搜索条件
const searchForm = reactive({
  keyword: '',
  category_id: null,
  status: null
})

// 分页
const pagination = reactive({
  page_num: 1,
  page_size: 10,
  total: 0
})

// 父分类列表（用于筛选下拉）
const parentCategories = ref([])

// ==================== 获取数据 ====================
onMounted(() => {
  fetchParentCategories()
  fetchProducts()
})

// 获取父分类列表
async function fetchParentCategories() {
  try {
    const res = await api.get('/product/category/parents')
    parentCategories.value = res.data || []
  } catch (e) {
    // ignore
  }
}

// 获取商品列表
async function fetchProducts() {
  loading.value = true
  try {
    const params = {
      page_num: pagination.page_num,
      page_size: pagination.page_size
    }
    if (searchForm.keyword) params.keyword = searchForm.keyword
    if (searchForm.category_id) params.category_id = searchForm.category_id
    if (searchForm.status !== null && searchForm.status !== '') params.status = searchForm.status

    const res = await api.get('/product/list', { params })
    const data = res.data
    productList.value = data.list || []
    pagination.total = data.total || 0
  } catch (e) {
    productList.value = []
  } finally {
    loading.value = false
  }
}

// 搜索
function handleSearch() {
  pagination.page_num = 1
  fetchProducts()
}

// 重置搜索
function resetSearch() {
  searchForm.keyword = ''
  searchForm.category_id = null
  searchForm.status = null
  pagination.page_num = 1
  fetchProducts()
}

// ==================== 新增/编辑弹窗 ====================
const dialogVisible = ref(false)
const submitLoading = ref(false)
const uploading = ref(false)
const formRef = ref(null)
const isEdit = ref(false)
const editId = ref(null)

// 用于图片上传的隐藏 input
const fileInput = ref(null)

const form = reactive({
  category_id: null,
  name: '',
  price: 99,
  stock: 100,
  keywords: '',
  image: '',
  status: 1
})

const formRules = {
  category_id: [{ required: true, message: '请选择分类', trigger: 'change' }],
  name: [{ required: true, message: '请输入商品名称', trigger: 'blur' }],
  price: [{ required: true, message: '请输入价格', trigger: 'blur' }],
  stock: [{ required: true, message: '请输入库存', trigger: 'blur' }]
}

// 分类级联选项（用于新增编辑表单）
const categoryOptions = ref([])

// 获取分类树（级联选择器用）
async function fetchCategoryTree() {
  try {
    const res = await api.get('/product/category/parents')
    const parents = res.data || []
    // 为每个父分类加载子分类
    const tree = []
    for (const parent of parents) {
      const node = { ...parent }
      try {
        const childRes = await api.get('/product/category/children', {
          params: { parent_id: parent.id }
        })
        node.children = (childRes.data || []).map(c => ({ ...c }))
      } catch (e) {
        node.children = []
      }
      tree.push(node)
    }
    categoryOptions.value = tree
  } catch (e) {
    categoryOptions.value = []
  }
}

const dialogTitle = computed(() => (isEdit.value ? '编辑商品' : '新增商品'))

// 打开新增弹窗
async function openAddDialog() {
  isEdit.value = false
  editId.value = null
  resetFormData()
  dialogVisible.value = true
  if (categoryOptions.value.length === 0) {
    await fetchCategoryTree()
  }
}

// 打开编辑弹窗
async function openEditDialog(row) {
  isEdit.value = true
  editId.value = row.id
  // 根据 category_id 查找对应的级联路径
  form.category_id = row.category_id
  form.name = row.name
  form.price = row.price
  form.stock = row.stock
  form.keywords = row.keywords || ''
  form.image = row.image || ''
  form.status = row.status

  dialogVisible.value = true
  if (categoryOptions.value.length === 0) {
    await fetchCategoryTree()
  }
}

// 重置表单数据
function resetFormData() {
  form.category_id = null
  form.name = ''
  form.price = 99
  form.stock = 100
  form.keywords = ''
  form.image = ''
  form.status = 1
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

// 关闭弹窗时重置
function resetForm() {
  resetFormData()
  isEdit.value = false
  editId.value = null
}

// 图片上传处理
function handleImageChange(file) {
  // el-upload 的 on-change 触发后，手动用 FormData 上传
  doUpload(file.raw)
}

// 触发上传（点击按钮时）
function triggerUpload() {
  // 创建一个隐藏的 file input
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = 'image/*'
  input.onchange = (e) => {
    const file = e.target.files[0]
    if (file) {
      doUpload(file)
    }
  }
  input.click()
}

// 执行上传
async function doUpload(file) {
  if (!file) return

  // 文件大小限制 5MB
  if (file.size > 5 * 1024 * 1024) {
    ElMessage.warning('图片大小不能超过5MB')
    return
  }

  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', file)
    const res = await api.post('/admin/upload', fd, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
    // 后端返回图片路径
    const imageUrl = res.data || res.data?.url || res.data?.path
    if (imageUrl) {
      form.image = imageUrl
      ElMessage.success('图片上传成功')
    } else {
      ElMessage.warning('上传成功但未获取到图片路径')
    }
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    uploading.value = false
  }
}

// 提交表单
async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitLoading.value = true
  try {
    const body = {
      category_id: form.category_id,
      name: form.name,
      price: form.price,
      stock: form.stock,
      keywords: form.keywords,
      image: form.image,
      status: form.status
    }

    if (isEdit.value) {
      await api.put(`/admin/product/${editId.value}`, body)
      ElMessage.success('商品更新成功')
    } else {
      await api.post('/admin/product', body)
      ElMessage.success('商品创建成功')
    }

    dialogVisible.value = false
    fetchProducts()
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    submitLoading.value = false
  }
}

// ==================== 删除商品 ====================
async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(
      `确定要删除商品「${row.name}」吗？此操作不可恢复。`,
      '删除确认',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await api.delete(`/admin/product/${row.id}`)
    ElMessage.success('商品已删除')
    fetchProducts()
  } catch (e) {
    if (e === 'cancel' || e === 'close') return
    // 错误已由拦截器处理
  }
}

// ==================== 批量生成关键词 ====================
const batchKeywordsLoading = ref(false)

async function handleBatchKeywords() {
  try {
    await ElMessageBox.confirm(
      '将为所有关键词为空的商品自动生成搜索关键词，是否继续？',
      '批量生成关键词',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'info'
      }
    )
    batchKeywordsLoading.value = true
    await api.post('/admin/product/batch-keywords')
    ElMessage.success('批量生成关键词已完成')
    fetchProducts()
  } catch (e) {
    if (e === 'cancel' || e === 'close') return
    // 错误已由拦截器处理
  } finally {
    batchKeywordsLoading.value = false
  }
}
</script>

<style scoped>
.product-manage {
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

.filter-card {
  margin-bottom: 16px;
}

.filter-form {
  display: flex;
  flex-wrap: wrap;
  gap: 0;
}

.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.upload-wrap {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.image-uploader {
  width: 148px;
  height: 148px;
  border: 1px dashed #d9d9d9;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: border-color 0.2s;
  overflow: hidden;
}

.image-uploader:hover {
  border-color: #409EFF;
}

.upload-icon {
  font-size: 28px;
  color: #8c939d;
}

.upload-tip {
  display: flex;
  align-items: center;
}

.image-actions {
  margin-top: 4px;
}
</style>
