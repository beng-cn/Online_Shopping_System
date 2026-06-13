<template>
  <div class="category-manage">
    <!-- 顶部栏 -->
    <el-card shadow="never" class="header-card">
      <div class="header-bar">
        <h2>分类管理</h2>
        <div class="header-right">
          <el-button type="success" @click="openAddDialog(null)">
            <el-icon><Plus /></el-icon> 新增分类
          </el-button>
          <el-button @click="fetchCategories">
            <el-icon><Refresh /></el-icon> 刷新
          </el-button>
        </div>
      </div>
    </el-card>

    <!-- 分类树形表格 -->
    <el-card shadow="never">
      <el-table
        v-loading="loading"
        :data="treeData"
        row-key="id"
        border
        stripe
        style="width: 100%"
        empty-text="暂无分类数据"
        :tree-props="{ children: 'children', hasChildren: 'hasChildren' }"
        default-expand-all
      >
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column prop="name" label="分类名称" min-width="200" show-overflow-tooltip />
        <el-table-column label="层级" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.parent_id === 0 ? 'primary' : 'info'" size="small">
              {{ row.parent_id === 0 ? '父分类' : '子分类' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="170" align="center" show-overflow-tooltip />
        <el-table-column label="操作" width="200" align="center" fixed="right">
          <template #default="{ row }">
            <el-button type="info" size="small" link @click="openEditDialog(row)">编辑</el-button>
            <el-button
              v-if="row.parent_id === 0"
              type="success"
              size="small"
              link
              @click="openAddDialog(row)"
            >
              添加子分类
            </el-button>
            <el-button type="danger" size="small" link @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新增/编辑分类弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="500px"
      :close-on-click-modal="false"
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="分类名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入分类名称" maxlength="50" />
        </el-form-item>
        <el-form-item label="父分类" prop="parent_id">
          <el-select v-model="form.parent_id" placeholder="无（作为父分类）" clearable style="width: 100%">
            <el-option
              v-for="cat in parentCategories"
              :key="cat.id"
              :label="cat.name"
              :value="cat.id"
            />
          </el-select>
          <div class="form-tip">留空则创建为父分类</div>
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="0">禁用</el-radio>
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
const treeData = ref([]) // 树形数据（含 children）
const parentCategories = ref([]) // 纯父分类列表（用于下拉选择）

onMounted(() => {
  fetchCategories()
})

// 获取分类树
async function fetchCategories() {
  loading.value = true
  try {
    // 获取父分类
    const parentRes = await api.get('/product/category/parents')
    const parents = parentRes.data || []
    parentCategories.value = parents

    // 为每个父分类获取子分类
    const tree = []
    for (const parent of parents) {
      const node = { ...parent }
      try {
        const childRes = await api.get('/product/category/children', {
          params: { parent_id: parent.id }
        })
        const children = childRes.data || []
        node.children = children.map(c => ({ ...c }))
        node.hasChildren = children.length > 0
      } catch (e) {
        node.children = []
        node.hasChildren = false
      }
      tree.push(node)
    }
    treeData.value = tree
  } catch (e) {
    treeData.value = []
    parentCategories.value = []
  } finally {
    loading.value = false
  }
}

// ==================== 新增/编辑弹窗 ====================
const dialogVisible = ref(false)
const submitLoading = ref(false)
const formRef = ref(null)
const isEdit = ref(false)
const editId = ref(null)
const presetParentId = ref(0) // 预设的父分类ID（用于添加子分类）

const form = reactive({
  name: '',
  parent_id: 0,
  status: 1
})

const formRules = {
  name: [{ required: true, message: '请输入分类名称', trigger: 'blur' }]
}

const dialogTitle = computed(() => (isEdit.value ? '编辑分类' : '新增分类'))

// 打开新增弹窗
function openAddDialog(parentRow) {
  isEdit.value = false
  editId.value = null
  form.name = ''
  form.parent_id = parentRow ? parentRow.id : 0
  form.status = 1
  presetParentId.value = parentRow ? parentRow.id : 0
  dialogVisible.value = true
}

// 打开编辑弹窗
function openEditDialog(row) {
  isEdit.value = true
  editId.value = row.id
  form.name = row.name
  form.parent_id = row.parent_id || 0
  form.status = row.status
  presetParentId.value = row.parent_id || 0
  dialogVisible.value = true
}

// 重置表单
function resetForm() {
  form.name = ''
  form.parent_id = 0
  form.status = 1
  isEdit.value = false
  editId.value = null
  presetParentId.value = 0
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

// 提交表单
async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitLoading.value = true
  try {
    const body = {
      name: form.name,
      parent_id: form.parent_id || 0,
      status: form.status
    }

    if (isEdit.value) {
      await api.put(`/admin/category/${editId.value}`, body)
      ElMessage.success('分类更新成功')
    } else {
      await api.post('/admin/category/add', body)
      ElMessage.success('分类创建成功')
    }

    dialogVisible.value = false
    fetchCategories()
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    submitLoading.value = false
  }
}

// ==================== 删除分类 ====================
async function handleDelete(row) {
  const hasChildren = row.children && row.children.length > 0
  const warningText = hasChildren
    ? `分类「${row.name}」下含有 ${row.children.length} 个子分类，删除后子分类也将被删除。确定要继续吗？`
    : `确定要删除分类「${row.name}」吗？此操作不可恢复。`

  try {
    await ElMessageBox.confirm(warningText, '删除确认', {
      confirmButtonText: '确定删除',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await api.delete(`/admin/category/${row.id}`)
    ElMessage.success('分类已删除')
    fetchCategories()
  } catch (e) {
    if (e === 'cancel' || e === 'close') return
    // 错误已由拦截器处理
  }
}
</script>

<style scoped>
.category-manage {
  max-width: 1200px;
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
