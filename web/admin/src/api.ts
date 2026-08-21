import type { Connect, Component, Group, Overview, Tool } from './types'

const TOKEN_KEY = 'mcphub-admin-token'

export class UnauthorizedError extends Error {}

export const getAdminToken = () => sessionStorage.getItem(TOKEN_KEY) || ''
export const setAdminToken = (token: string) => sessionStorage.setItem(TOKEN_KEY, token)
export const clearAdminToken = () => sessionStorage.removeItem(TOKEN_KEY)

type RequestOptions = RequestInit & { token?: string; redirectOnUnauthorized?: boolean }

export const request = async <T,>(path: string, options: RequestOptions = {}): Promise<T> => {
  const { token = getAdminToken(), redirectOnUnauthorized = true, headers, ...init } = options
  const requestHeaders = new Headers(headers)
  if (token) requestHeaders.set('Authorization', `Bearer ${token}`)
  const response = await fetch(path, { ...init, headers: requestHeaders })
  if (response.status === 401) {
    clearAdminToken()
    if (redirectOnUnauthorized && !location.pathname.startsWith('/admin/login')) {
      const redirect = `${location.pathname}${location.search}`
      location.assign(`/admin/login?redirect=${encodeURIComponent(redirect)}`)
    }
    throw new UnauthorizedError('Admin authentication required')
  }
  if (!response.ok) throw new Error((await response.text()) || `${response.status} ${response.statusText}`)
  if (response.status === 204 || response.status === 202) return undefined as T
  return response.json() as Promise<T>
}

export const validateAdmin = (token = '') => request<Overview>('/api/admin/overview', { token, redirectOnUnauthorized: false })
export const getOverview = (signal?: AbortSignal) => request<Overview>('/api/admin/overview', { signal })
export const getConnects = async (signal?: AbortSignal) => (await request<{ connects: Connect[] }>('/api/admin/connects', { signal })).connects
export const getComponents = async (signal?: AbortSignal) => (await request<{ components: Component[] }>('/api/admin/components', { signal })).components
export const getComponent = (id: string, signal?: AbortSignal) => request<Component>(`/api/admin/components/${encodeURIComponent(id)}`, { signal })
export const getGroups = async (signal?: AbortSignal) => (await request<{ groups: Group[] }>('/api/admin/groups', { signal })).groups
export const getGroupTools = async (id: string, signal?: AbortSignal) => (await request<{ tools: Tool[] }>(`/api/admin/groups/${encodeURIComponent(id)}/tools`, { signal })).tools
export const createGroup = (group: Group) => request<Group>('/api/admin/groups', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(group) })
export const deleteGroup = (id: string) => request<void>(`/api/admin/groups/${encodeURIComponent(id)}`, { method: 'DELETE' })
export const attachTool = (group: string, component: string, tool: string) => request<void>(`/api/admin/groups/${encodeURIComponent(group)}/tools`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ component_id: component, tool_name: tool }) })
export const detachTool = (group: string, component: string, tool: string) => request<void>(`/api/admin/groups/${encodeURIComponent(group)}/tools/${encodeURIComponent(component)}/${encodeURIComponent(tool)}`, { method: 'DELETE' })
export const refreshCatalog = (component: string) => request<void>(`/api/admin/catalog/components/${encodeURIComponent(component)}/refresh`, { method: 'POST' })
export const getTokenGroups = (tokenId: string, signal?: AbortSignal) => request<{ tenant_id: string; default_group_id: string; group_ids: string[] }>(`/api/admin/tokens/${encodeURIComponent(tokenId)}/groups`, { signal })
export const setTokenGroups = (tokenId: string, defaultGroupId: string, groupIds: string[]) => request<void>(`/api/admin/tokens/${encodeURIComponent(tokenId)}/groups`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ default_group_id: defaultGroupId, group_ids: groupIds }) })
