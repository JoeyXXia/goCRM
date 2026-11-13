import request from '@/utils/request'

export interface CustomerInput { name: string; industry: string; remark: string }
export interface ContactInput { name: string; phone: string; email: string; title: string }
export interface CustomerRecord extends CustomerInput {
  id: string
  tenantId: string
  code: string
  nextFollowupAt: string | null
  ownerId: string
  departmentId: string
}
export interface Customer extends CustomerRecord { ownerName: string; departmentName: string }
export interface CustomerDetail extends Customer { canUpdate: boolean; canDelete: boolean; canTransfer: boolean }
export interface Contact extends ContactInput { id: string; tenantId: string; customerId: string; masked: boolean }
export interface CustomerPage { list: Customer[]; total: number; page: number; pageSize: number }
export interface CustomerSearch { page: number; pageSize: number; sort: string; order: string; keyword: string; followup: string }

export interface Followup { id: string; actorName: string; content: string; createdAt: string }
export interface FollowupPage { list: Followup[]; total: number; page: number; pageSize: number }
export interface TransferTarget { userId: string; name: string; departmentName: string }
export interface AuditEvent { id: string; action: string; actorName: string; createdAt: string; fromOwnerId: string; toOwnerId: string }

/** Reuses GVA authentication and sends an explicit tenant selection on every request. */
async function crm<T>(tenantId: string, url: string, signal: AbortSignal, method = 'get', data?: unknown, params?: unknown): Promise<T> {
  const config = {
    url: '/dealflow' + url, method, data, params, signal,
    headers: { 'X-Tenant-ID': tenantId }, donNotShowLoading: true
  }
  const result: { code: number; data: T; msg: string } = await request(config)
  if (result.code !== 0) throw new Error(result.msg)
  return result.data
}
export const listCustomers = (tenant: string, search: CustomerSearch, signal: AbortSignal) =>
  crm<CustomerPage>(tenant, '/customers', signal, 'get', undefined, search)
export const getCustomer = (tenant: string, id: string, signal: AbortSignal) =>
  crm<CustomerDetail>(tenant, '/customers/' + encodeURIComponent(id), signal)
export const createCustomer = (tenant: string, input: CustomerInput, signal: AbortSignal) =>
  crm<CustomerRecord>(tenant, '/customers', signal, 'post', input)
export const updateCustomer = (tenant: string, id: string, input: CustomerInput, signal: AbortSignal) =>
  crm<null>(tenant, '/customers/' + encodeURIComponent(id), signal, 'put', input)
export const deleteCustomer = (tenant: string, id: string, signal: AbortSignal) =>
  crm<null>(tenant, '/customers/' + encodeURIComponent(id), signal, 'delete')
export const listContacts = (tenant: string, id: string, signal: AbortSignal) =>
  crm<Contact[]>(tenant, '/customers/' + encodeURIComponent(id) + '/contacts', signal)
export const createContact = (tenant: string, id: string, input: ContactInput, signal: AbortSignal) =>
  crm<Contact>(tenant, '/customers/' + encodeURIComponent(id) + '/contacts', signal, 'post', input)
export const updateContact = (tenant: string, id: string, input: ContactInput, signal: AbortSignal) =>
  crm<null>(tenant, '/contacts/' + encodeURIComponent(id), signal, 'put', input)
export const deleteContact = (tenant: string, id: string, signal: AbortSignal) =>
  crm<null>(tenant, '/contacts/' + encodeURIComponent(id), signal, 'delete')

export const listFollowups = (tenant: string, id: string, page: number, signal: AbortSignal) =>
  crm<FollowupPage>(tenant, '/customers/' + encodeURIComponent(id) + '/followups', signal, 'get', undefined, { page, pageSize: 20 })
export const addFollowup = (tenant: string, id: string, input: { content: string; nextFollowupAt: string | null }, signal: AbortSignal) =>
  crm<Followup>(tenant, '/customers/' + encodeURIComponent(id) + '/followups', signal, 'post', input)
export const listTransferTargets = (tenant: string, id: string, signal: AbortSignal) =>
  crm<TransferTarget[]>(tenant, '/customers/' + encodeURIComponent(id) + '/transfer-targets', signal)
export const transferCustomer = (tenant: string, id: string, ownerId: string, signal: AbortSignal) =>
  crm<null>(tenant, '/customers/' + encodeURIComponent(id) + '/transfer', signal, 'post', { ownerId })
export const listCustomerAudit = (tenant: string, id: string, signal: AbortSignal) =>
  crm<AuditEvent[]>(tenant, '/customers/' + encodeURIComponent(id) + '/audit', signal)
