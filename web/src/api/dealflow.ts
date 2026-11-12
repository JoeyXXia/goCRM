import request from '@/utils/request'

export interface Workspace {
  id: string
  name: string
  departmentId: string
  roleKey: string
}

export interface MenuItem {
  key: string
  canRename: boolean
  canCreate?: boolean
}

export interface Department {
  name: string
  code: string
}

interface Envelope<T> {
  code: number
  data: T
  msg: string
}

async function getData<T>(url: string, signal: AbortSignal, tenantId?: string, name?: string): Promise<T> {
  const config = {
    url,
    signal,
    headers: tenantId ? { 'X-Tenant-ID': tenantId } : undefined,
    method: name === undefined ? 'get' : 'patch',
    data: name === undefined ? undefined : { name },
    donNotShowLoading: true
  }
  const response: Envelope<T> = await request(config)
  if (response.code !== 0) throw new Error(response.msg)
  return response.data
}

export const getWorkspaces = (signal: AbortSignal) =>
  getData<Workspace[]>('/dealflow/workspaces', signal)

export const getMenu = (tenantId: string, signal: AbortSignal) =>
  getData<MenuItem[]>('/dealflow/menu', signal, tenantId)

export const getDepartment = (tenantId: string, id: string, signal: AbortSignal) =>
  getData<Department>(`/dealflow/departments/${encodeURIComponent(id)}`, signal, tenantId)

export const renameDepartment = (tenantId: string, id: string, name: string, signal: AbortSignal) =>
  getData<null>(`/dealflow/departments/${encodeURIComponent(id)}`, signal, tenantId, name)
