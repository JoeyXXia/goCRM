<template>
  <section aria-label="客户详情">
    <RouterLink :to="{ name: 'DealflowCustomers' }">返回客户列表</RouterLink>
    <p v-if="loading" role="status">正在加载客户…</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <template v-if="customer">
      <h2 class="my-3 text-lg font-semibold">{{ customer.name }}</h2>
      <p>业务编号 {{ customer.code }}</p>
      <p class="mb-4">负责人 {{ customer.ownerName }} · 部门 {{ customer.departmentName }}</p>
      <form v-if="customer.canUpdate" aria-label="编辑客户" class="grid max-w-xl gap-2" @submit.prevent="saveCustomer">
        <label for="edit-customer-name">客户名称</label><el-input id="edit-customer-name" v-model="draft.name" required maxlength="128" />
        <label for="edit-customer-industry">行业</label><el-input id="edit-customer-industry" v-model="draft.industry" maxlength="64" />
        <label for="edit-customer-remark">备注</label><el-input id="edit-customer-remark" v-model="draft.remark" type="textarea" maxlength="512" />
        <el-button type="primary" native-type="submit" :loading="saving">保存客户</el-button>
      </form>
      <template v-else><p>行业 {{ customer.industry || '未填写' }}</p><p>备注 {{ customer.remark || '未填写' }}</p></template>
      <h3 class="mb-3 mt-6 font-semibold">联系人</h3>
      <el-table :data="contacts" row-key="id" empty-text="暂无联系人">
        <el-table-column label="姓名" prop="name" />
        <el-table-column label="电话" prop="phone" />
        <el-table-column label="邮箱" prop="email" />
        <el-table-column label="职位" prop="title" />
        <el-table-column v-if="customer.canUpdate" label="操作">
          <template #default="{ row }">
            <el-button :disabled="saving" @click="editContact(row)">编辑</el-button>
            <el-button :disabled="saving" type="danger" @click="removeContact(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <form v-if="customer.canUpdate" aria-label="联系人表单" class="mt-4 grid max-w-xl gap-2" @submit.prevent="saveContact">
        <h4>{{ editingContact ? '编辑联系人' : '新增联系人' }}</h4>
        <label for="contact-name">联系人姓名</label><el-input id="contact-name" v-model="contactDraft.name" required maxlength="128" />
        <label for="contact-phone">联系人电话</label><el-input id="contact-phone" v-model="contactDraft.phone" maxlength="32" />
        <label for="contact-email">联系人邮箱</label><el-input id="contact-email" v-model="contactDraft.email" type="email" maxlength="254" />
        <label for="contact-title">联系人职位</label><el-input id="contact-title" v-model="contactDraft.title" maxlength="64" />
        <div class="flex gap-3">
          <el-button type="primary" native-type="submit" :loading="saving">保存联系人</el-button>
          <el-button v-if="editingContact" :disabled="saving" @click="resetContact">取消编辑</el-button>
        </div>
      </form>
      <el-button v-if="customer.canDelete" class="mt-6" type="danger" :disabled="saving || contacts.length > 0" @click="removeCustomer">删除客户</el-button>
      <p v-if="customer.canDelete && contacts.length">移除联系人后可删除客户。</p>
    </template>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { createContact, deleteContact, deleteCustomer, getCustomer, listContacts, updateContact, updateCustomer, type Contact, type CustomerDetail } from '@/api/dealflow-crm'
import { useDealflowSession } from '@/pinia/modules/dealflow-session'
const props = defineProps<{ tenantId: string }>()
const route = useRoute(), router = useRouter(), session = useDealflowSession()
const id = String(route.params.id)
const customer = ref<CustomerDetail>()
const contacts = ref<Contact[]>([])
const draft = reactive({ name: '', industry: '', remark: '' })
const contactDraft = reactive({ name: '', phone: '', email: '', title: '' })
const editingContact = ref('')
const loading = ref(true), saving = ref(false), error = ref('')
const request = new AbortController()
watch(() => session.menu, () => request.abort(), { flush: 'sync' })
onBeforeUnmount(() => request.abort())
onMounted(load)
async function load() {
  loading.value = true
  try {
    const [row, people] = await Promise.all([getCustomer(props.tenantId, id, request.signal), listContacts(props.tenantId, id, request.signal)])
    if (request.signal.aborted) return
    customer.value = row
    contacts.value = people
    Object.assign(draft, { name: row.name, industry: row.industry, remark: row.remark })
  } catch {
    if (!request.signal.aborted) { customer.value = undefined; contacts.value = []; error.value = '客户不存在、无权访问或暂时无法加载。' }
  } finally { if (!request.signal.aborted) loading.value = false }
}
async function mutate(action: () => Promise<unknown>) {
  if (saving.value || request.signal.aborted) return
  saving.value = true
  error.value = ''
  try {
    await action()
    if (!request.signal.aborted) await load()
  } catch { if (!request.signal.aborted) error.value = '无法保存，请检查表单和当前权限。' }
  finally { if (!request.signal.aborted) saving.value = false }
}
async function saveCustomer() {
  if (!customer.value?.canUpdate) return
  await mutate(() => updateCustomer(props.tenantId, id, { ...draft }, request.signal))
}
function resetContact() {
  editingContact.value = ''
  Object.assign(contactDraft, { name: '', phone: '', email: '', title: '' })
}
function editContact(row: Contact) {
  editingContact.value = row.id
  Object.assign(contactDraft, { name: row.name, phone: row.phone, email: row.email, title: row.title })
}
async function saveContact() {
  if (!customer.value?.canUpdate) return
  await mutate(async () => {
    if (editingContact.value) await updateContact(props.tenantId, editingContact.value, { ...contactDraft }, request.signal)
    else await createContact(props.tenantId, id, { ...contactDraft }, request.signal)
    if (!request.signal.aborted) resetContact()
  })
}
async function removeContact(contactId: string) {
  try { await ElMessageBox.confirm('确认删除这位联系人？', '删除联系人', { type: 'warning' }) } catch { return }
  if (!customer.value?.canUpdate) return
  await mutate(() => deleteContact(props.tenantId, contactId, request.signal))
}
async function removeCustomer() {
  try { await ElMessageBox.confirm('确认删除这个客户？', '删除客户', { type: 'warning' }) } catch { return }
  if (saving.value || request.signal.aborted || !customer.value?.canDelete) return
  saving.value = true
  try {
    await deleteCustomer(props.tenantId, id, request.signal)
    if (!request.signal.aborted) await router.replace({ name: 'DealflowCustomers' })
  } catch { if (!request.signal.aborted) error.value = '无法删除客户，请先移除联系人并检查当前权限。' }
  finally { if (!request.signal.aborted) saving.value = false }
}
</script>
